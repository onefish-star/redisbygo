package server

import (
	"bufio"
	"fmt"
	"io"
	"net"
	constructor2 "redis/app/server/constructor"
	"redis/pkg/client"
	"redis/pkg/client/constructor"
	config2 "redis/pkg/config"
	"redis/pkg/ds/robj"
	"redis/pkg/redisdb/redisdb"
	"redis/pkg/utils"
	"strconv"
)

func InitServerConfig(s *constructor2.Server) {
	s.Log.Info("oO0OoO0OoO0Oo Redis is starting oO0OoO0OoO0Oo")
}

func CreateServer() {
	s := constructor2.NewServer()
	c := config2.NewConfig()
	InitServerConfig(s)

	s.Log.Info("server start")
	s.Log.Info("serve on :" + strconv.Itoa(c.Port))

	ln, err := net.Listen("tcp", ":"+strconv.Itoa(c.Port))

	s.Listener = ln
	if err != nil {
		panic(err)
	}
	go acceptRequest(s)
	go handleCommands(s)
}

// 处理命令
func handleCommands(s *constructor2.Server) {
	for {
		fmt.Println("waiting commands....")
		select {
		case command := <-s.Commands:
			fmt.Println("handleCommands", command.Query)
			// 解析命令
			parseCommand(command)
			// 回复
			response(command.Conn, "success")
			// 写入aof
			s.Aof.Write(command.Query)
		}
	}
}

// 解析命令
func parseCommand(c *client.Client) {
	key := robj.NewRedisObject()
	redisdb.Add(c.Db, key, key)
}

// 回复客户端
func response(conn net.Conn, message string) {
	writer := bufio.NewWriter(conn)
	_, _ = writer.WriteString(utils.ProtocolLine(message))
	_ = writer.Flush()
}

// 接受客户端请求
func acceptRequest(s *constructor2.Server) {
	for {
		s.Log.Info("waiting connecting...")
		conn, err := s.Listener.Accept()

		if err != nil {
			fmt.Println(err)
			s.Log.Info(err.Error())
		}

		s.No = s.No + 1
		newClient := constructor.NewClient(conn)
		newClient.Index = s.No
		newClient.Db = s.Db[0]
		cc := config2.NewConfig()

		if len(s.Clients) >= cc.Maxclients {
			w := bufio.NewWriter(newClient.Conn)
			_, _ = w.WriteString(utils.ProtocolLineErr("ERR max number of clients reached"))
			s.StatRejectedConn++
			_ = w.Flush()

			fmt.Println("client up to max")
		} else {
			s.Clients[s.No] = newClient
			fmt.Println("accept client::")

			go handleConnection(s, newClient)
		}
	}
}

// 处理客户端连接
func handleConnection(s *constructor2.Server, cl *client.Client) {
	s.Log.Info("new client")
	s.Log.Info(cl.Conn.RemoteAddr().String())

	c := 1024
	buf := make([]byte, c)
	for {
		size, err := cl.Conn.Read(buf)
		fmt.Println("size::", size, "err::", err)
		if size == 0 && err == io.EOF {
			// 客户端关闭
			err = cl.Conn.Close()
			if err != nil {
				fmt.Println("close client fail::", err)
			} else {
				fmt.Println("close client success::")
			}
			// 删除客户端
			delete(s.Clients, cl.Index)
			// 结束循环，回收协程
			break
		} else {
			fmt.Println("handleConnection", string(buf))
			// 发送客户端到单个协程，由单个协程处理
			cl.Query = string(buf)
			s.Commands <- cl
			buf = make([]byte, c)
		}
	}
}
