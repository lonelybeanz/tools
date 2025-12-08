package pcm

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/spf13/cast"
)

func TestNewSvr(t *testing.T) {
	s, _ := NewSvr("", func(ctx context.Context, msg interface{}, num int) (resp interface{}, err error) {
		fmt.Println(msg)
		time.Sleep(3 * time.Second)
		return "2", nil
	}, nil)

	s.Go()

	go func() {
		for i := 0; i < 10; i++ {
			c, cannel := context.WithTimeout(context.Background(), 1*time.Second)
			defer cannel()
			d, err := PostMsgToServer(c, s.Name, "1")
			fmt.Println(d, err)
		}
	}()

	time.Sleep(10 * time.Second)

	s.Stop()
}

type AServer struct {
	*Server
	A int
}

func (A *AServer) ServerName() string {
	//TODO implement me
	return "123"
}

func (A *AServer) MsgAction(ctx context.Context, msg interface{}, num int) (resp interface{}, err error) {
	fmt.Println(msg)
	return nil, err
}

func (A *AServer) ActionGoroutineNum() int {
	return 1
}

func (A *AServer) Schedule() []TimedTask {
	return nil
}

func (A *AServer) SetServer(s *Server) {
	A.Server = s
}

type UM struct {
	h string
}

func (um *UM) Hash() string {
	return um.h
}

func (um *UM) Unique() bool {
	return true
}

func TestNewMs(t *testing.T) {
	s := &AServer{}
	Init(s, WithOptionDeduplicate())

	s.Go()
	for i := 0; i < 10; i++ {
		msg := &UM{h: cast.ToString(i / 2)}
		s.PushMsgToServer(context.Background(), msg)
	}

	time.Sleep(1 * time.Second)
	s.Stop()
}
