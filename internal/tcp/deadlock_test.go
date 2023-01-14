package tcp

import (
	"fmt"
	"log"
	"net"
	"sync"
	"testing"
	"time"
)

func TestDeadlock(t *testing.T) {
	var wg sync.WaitGroup
	wg.Add(1)

	wait := make(chan struct{}, 1)

	go func() {
		defer wg.Done()

		l, err := net.Listen("tcp", "127.0.0.1:11011")
		if err != nil {
			t.Error(err)
		}

		go func() {
			<-time.After(10 * time.Second)

			l.Close()
		}()

		wait <- struct{}{}

		for {
			c, err := l.Accept()
			if err != nil {
				t.Error(err)
				break
			}

			log.Println("New client")
			go func(cc net.Conn) {
				for {
					conn := tcpConn{conn: cc}
					data, err := conn.Read()
					if err != nil {
						t.Error(err)
						return
					}

					log.Println("Ping:", string(data))

					time.Sleep(1)

					_, err = conn.Write([]byte("Pong: " + string(data)))
					if err != nil {
						t.Error(err)
						return
					}
				}
			}(c)
		}
	}()

	<-wait

	pool, _ := CreateTcpConnPool(&TcpConfig{
		Host:         "127.0.0.1",
		Port:         11011,
		MaxIdleConns: 1,
		MaxOpenConn:  1,
	})

	for i := 0; i < 2; i++ {
		wg.Add(1)

		go func(k int) {
			log.Println("Send ping:", k)

			resp, err := pool.SendData([]byte(fmt.Sprintf("Salom [%d]", k)))
			if err != nil {
				t.Error(err)
			}

			log.Println(string(resp))
			wg.Done()
		}(i)
	}

	wg.Wait()
}
