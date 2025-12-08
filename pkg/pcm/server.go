package pcm

/**
 * @Description: Producer-Consumer Model
 **/

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/zeromicro/go-zero/core/logx"
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
		go func() {
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
							resp, err := server.MsgActioner(req.ctx, req.msg, number)
							if req.sync {
								req.err = err
								req.ch <- resp
							}
							v, ok := req.msg.(UniqueMsg)
							if ok {
								server.Cache.Remove(v.Hash())
							}

						}
					}
				case <-server.MsgActionerExit[number]:
					return
				}
			}
		}()
	}
	for j := 0; j < server.ScheduledTaskGoroutineNum; j++ {
		number := j
		go func() {
			server.TimedTasks[number].Task(number)
			for {
				select {
				case <-server.ScheduledTaskerExit[number]:
					logx.Infof("[%s][%d] ScheduledTasker goroutine 退出\n", server.Name, number)
					return
				case <-time.After(server.TimedTasks[number].Time):
					server.TimedTasks[number].Task(number)
				}
			}
		}()
	}

	server.State = Started
}

func (server *Server) Stop() {
	if server.State == Started {

		server.State = Stopped

		for i := 0; i < server.ActionGoroutineNum; i++ {
			server.MsgActionerExit[i] <- 1
		}
		for i := 0; i < server.ScheduledTaskGoroutineNum; i++ {
			server.ScheduledTaskerExit[i] <- 1
		}

		close(server.Queue)

		ServersNew.Delete(server.Name)
	}
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
	if server.State == Stopped {
		return fmt.Errorf("server is stopped")
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
