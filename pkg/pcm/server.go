package pcm

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/lonelybeanz/tools/pkg/log"
)

const QueueLen = 100000

const TTL = 10 * time.Minute

var ServersNew = sync.Map{}

type GoState int

const (
	// Stopped server is stopped
	Stopped GoState = iota
	// Started server is started
	Started
	// Stopping server is in the process of stopping
	Stopping
)

type MsgAction func(ctx context.Context, msg interface{}, num int) (resp interface{}, err error)
type ScheduledTask func(num int)

type Req struct {
	ch   chan interface{}
	msg  interface{}
	err  error
	ctx  context.Context
	sync bool
}

type Server struct {
	Name                      string
	Opts                      Options
	Queue                     chan *Req
	State                     GoState
	ActionGoroutineNum        int
	MsgActionerExit           []chan int
	MsgActioner               MsgAction
	ScheduledTaskGoroutineNum int
	ScheduledTaskerExit       []chan int
	ScheduledTaskers          []ScheduledTask
	ScheduleTime              time.Duration
	TimedTasks                []TimedTask
	Cache                     *expirable.LRU[string, struct{}]

	// 任务完成状态跟踪
	pendingTasks  int64          // 正在处理的任务数量
	totalTasks    int64          // 总任务数量
	taskMutex     sync.RWMutex   // 保护任务计数器的互斥锁
	stopWaitGroup sync.WaitGroup // 等待所有任务完成
	stopChan      chan struct{}  // 停止通知通道
}

type TimedTask struct {
	Task ScheduledTask
	Time time.Duration
}

type Options struct {
	lastonly    bool
	deduplicate bool
}

type Option func(*Options) error

func WithOptionLastOnly() Option {
	return func(opt *Options) error {
		opt.lastonly = true
		return nil
	}
}

func WithOptionDeduplicate() Option {
	return func(opt *Options) error {
		opt.deduplicate = true
		return nil
	}
}

type UniqueMsg interface {
	Hash() string
	Unique() bool
}

func NewSvr(serverName string, action MsgAction, timedTasks []TimedTask, opts ...Option) (server *Server, err error) {

	if serverName == "" {
		serverName = RandomString(16)
	}
	// cache, _ := lru.New[string, struct{}](QueueLen)
	cache := expirable.NewLRU[string, struct{}](QueueLen, nil, TTL)
	server = &Server{
		Name:                      serverName,
		ActionGoroutineNum:        runtime.NumCPU() * 2,
		Queue:                     make(chan *Req, QueueLen),
		MsgActionerExit:           []chan int{},
		MsgActioner:               action,
		ScheduledTaskGoroutineNum: len(timedTasks),
		ScheduledTaskerExit:       []chan int{},
		TimedTasks:                timedTasks,
		Cache:                     cache,
		pendingTasks:              0,
		totalTasks:                0,
		stopChan:                  make(chan struct{}),
	}
	_, ok := ServersNew.LoadOrStore(serverName, server)
	if ok {
		return nil, fmt.Errorf("the same server serverName already exists:%s", serverName)
	}

	for _, opt := range opts {
		if err = opt(&server.Opts); err != nil {
			return nil, err
		}
	}
	if server.Opts.lastonly {
		server.ActionGoroutineNum = 1
	}

	if action == nil {
		server.ActionGoroutineNum = 0
	}

	server.State = Stopped
	return server, nil
}

func StopServer(serverName string) error {
	load, ok := ServersNew.Load(serverName)
	if !ok {
		return fmt.Errorf("the server serverName is not exists:%s", serverName)
	}
	s := load.(*Server)
	s.Stop()
	return nil
}

func PostMsgToServer(ctx context.Context, serverName string, msg interface{}) (interface{}, error) {
	load, ok := ServersNew.Load(serverName)
	if !ok {
		return nil, fmt.Errorf("server name doesn't exist:%s", serverName)
	}
	s := load.(*Server)

	return s.PostMsgToServer(ctx, msg)
}

func PushMsgToServer(ctx context.Context, serverName string, msg interface{}) error {
	load, ok := ServersNew.Load(serverName)
	if !ok {
		return fmt.Errorf("server name doesn't exist:%s", serverName)
	}
	s := load.(*Server)

	return s.PushMsgToServer(ctx, msg)
}

func (server *Server) Go() {
	for i := 0; i < server.ActionGoroutineNum; i++ {
		server.MsgActionerExit = append(server.MsgActionerExit, make(chan int))
	}
	for i := 0; i < server.ScheduledTaskGoroutineNum; i++ {
		server.ScheduledTaskerExit = append(server.ScheduledTaskerExit, make(chan int))
	}

	for i := 0; i < server.ActionGoroutineNum; i++ {
		number := i
		server.stopWaitGroup.Add(1)
		go func() {
			defer server.stopWaitGroup.Done()
			for {
				select {
				case req := <-server.Queue:
					{
						if server.Opts.lastonly {
							for {
								if len(server.Queue) > 1 {
									<-server.Queue
								} else if len(server.Queue) == 1 {
									req = <-server.Queue
									break
								} else {
									break
								}
							}
						}
						if server.MsgActioner != nil {
							// 增加正在处理的任务计数
							atomic.AddInt64(&server.pendingTasks, 1)

							resp, err := server.MsgActioner(req.ctx, req.msg, number)
							if req.sync {
								req.err = err
								req.ch <- resp
							}
							v, ok := req.msg.(UniqueMsg)
							if ok {
								server.Cache.Remove(v.Hash())
							}

							// 减少正在处理的任务计数
							atomic.AddInt64(&server.pendingTasks, -1)
						}
					}
				case <-server.MsgActionerExit[number]:
					return
				case <-server.stopChan:
					return
				}
			}
		}()
	}
	for j := 0; j < server.ScheduledTaskGoroutineNum; j++ {
		number := j
		server.stopWaitGroup.Add(1)
		go func() {
			defer server.stopWaitGroup.Done()
			server.TimedTasks[number].Task(number)
			for {
				select {
				case <-server.ScheduledTaskerExit[number]:
					log.Infof("[%s][%d] ScheduledTasker goroutine 退出\n", server.Name, number)
					return
				case <-time.After(server.TimedTasks[number].Time):
					server.TimedTasks[number].Task(number)
				case <-server.stopChan:
					return
				}
			}
		}()
	}

	server.State = Started
}

func (server *Server) Stop() {
	if server.State == Started || server.State == Stopping {
		server.State = Stopped

		// 通知所有goroutine退出
		for i := 0; i < server.ActionGoroutineNum; i++ {
			server.MsgActionerExit[i] <- 1
		}
		for i := 0; i < server.ScheduledTaskGoroutineNum; i++ {
			server.ScheduledTaskerExit[i] <- 1
		}

		// 关闭停止通道
		close(server.stopChan)

		// 等待所有goroutine完成
		server.stopWaitGroup.Wait()

		close(server.Queue)

		ServersNew.Delete(server.Name)
		log.Infof("%s server stop!!!!", server.Name)
	}
}

// StopAfterDone 等待所有任务完成后停止服务器
func (server *Server) StopAfterDone(timeout time.Duration) error {
	if server.State != Started {
		return fmt.Errorf("server is not started")
	}

	server.State = Stopping

	// 创建超时上下文
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	// 等待所有任务完成或超时
	for {
		select {
		case <-ctx.Done():
			// 超时，强制停止
			log.Errorf("%s server stop timeout, force stopping...", server.Name)
			server.Stop()
			return fmt.Errorf("stop timeout after %v", timeout)
		default:
			// 检查是否所有任务都已完成
			if server.IsAllTasksDone() {
				log.Infof("%s all tasks completed, stopping server...", server.Name)
				server.Stop()
				return nil
			}
			// 短暂等待后继续检查
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// IsAllTasksDone 检查是否所有任务都已完成
func (server *Server) IsAllTasksDone() bool {
	server.taskMutex.RLock()
	defer server.taskMutex.RUnlock()

	pending := atomic.LoadInt64(&server.pendingTasks)
	queueLen := len(server.Queue)

	return pending == 0 && queueLen == 0
}

// GetTaskStats 获取任务统计信息
func (server *Server) GetTaskStats() (pending, total int64) {
	server.taskMutex.RLock()
	defer server.taskMutex.RUnlock()

	pending = atomic.LoadInt64(&server.pendingTasks)
	total = atomic.LoadInt64(&server.totalTasks)

	return pending, total
}

// GetPendingTaskCount 获取正在处理的任务数量
func (server *Server) GetPendingTaskCount() int64 {
	return atomic.LoadInt64(&server.pendingTasks)
}

// GetTotalTaskCount 获取总任务数量
func (server *Server) GetTotalTaskCount() int64 {
	return atomic.LoadInt64(&server.totalTasks)
}

// ResetTaskStats 重置任务统计信息
func (server *Server) ResetTaskStats() {
	server.taskMutex.Lock()
	defer server.taskMutex.Unlock()

	atomic.StoreInt64(&server.pendingTasks, 0)
	atomic.StoreInt64(&server.totalTasks, 0)
}

func (server *Server) PostMsgToServer(ctx context.Context, msg interface{}) (resp interface{}, err error) {
	if server.State == Stopped {
		return nil, fmt.Errorf("server is stopped")
	}

	if server.Opts.lastonly {
		return nil, fmt.Errorf("lastonly does not support func PostMsgToServer")
	}
	defer func() {
		if recover() != nil {
			resp = nil
			err = fmt.Errorf("server is stopped")
			return
		}
	}()

	if server.Opts.deduplicate {
		v, ok := msg.(UniqueMsg)
		if ok {
			if v.Unique() && server.Cache.Contains(v.Hash()) {
				return nil, fmt.Errorf("unique msg is exist")
			} else {
				server.Cache.Add(v.Hash(), struct{}{})
			}
		}
	}

	// 增加总任务计数
	atomic.AddInt64(&server.totalTasks, 1)

	req := &Req{ch: make(chan interface{}), msg: msg, err: nil, ctx: ctx, sync: true}
	server.Queue <- req
	select {
	case resp = <-req.ch:
		close(req.ch)
		return resp, req.err
	case <-ctx.Done():
		go func(req *Req) {
			<-req.ch
			close(req.ch)
		}(req)
	}
	return nil, ctx.Err()
}

func (server *Server) PushMsgToServer(ctx context.Context, msg interface{}) (err error) {
	if server.State == Stopped || server.State == Stopping {
		return fmt.Errorf("server is stopped or stopping")
	}
	defer func() {
		if recover() != nil {
			err = fmt.Errorf("server is stopped")
			return
		}
	}()

	if server.Opts.deduplicate {
		v, ok := msg.(UniqueMsg)
		if ok {
			if v.Unique() && server.Cache.Contains(v.Hash()) {
				fmt.Errorf("deduplicate")
				return fmt.Errorf("unique msg is exist")
			} else {
				server.Cache.Add(v.Hash(), struct{}{})
			}
		}
	}

	// 增加总任务计数
	atomic.AddInt64(&server.totalTasks, 1)

	req := &Req{ch: nil, msg: msg, err: nil, ctx: ctx, sync: false}
	server.Queue <- req
	return nil
}

func RandomString(length int) string {
	b := make([]byte, length)
	_, err := rand.Read(b)
	if err != nil {
		panic(err)
	}
	return base64.StdEncoding.EncodeToString(b)[:length]
}

type IServer interface {
	ServerName() string
	MsgAction(ctx context.Context, msg interface{}, num int) (resp interface{}, err error)
	ActionGoroutineNum() int
	Schedule() []TimedTask
	SetServer(s *Server)
}

func Init(s IServer, option ...Option) (*Server, error) {
	server, err := NewSvr(s.ServerName(), s.MsgAction, s.Schedule(), option...)
	if err != nil {
		return nil, err
	}
	server.ActionGoroutineNum = s.ActionGoroutineNum()
	s.SetServer(server)
	return server, nil
}
