package main

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

var (
	log = logrus.New()
	SS  ServerSlice
)

type Server struct {
	Server     string  `json:"server"`
	ServerPort int     `json:"server_port"`
	Password   string  `json:"password"`
	Method     string  `json:"method"`
	Remarks    string  `json:"remarks"`
	Speed      float64 `json:"speed"`
	Auth       bool    `json:"auth"`
}

type ServerSlice struct {
	Servers       []Server `json:"servers"`
	TestURL       string   `json:"testURL"`
	OutFile       string   `json:"outFile"`
	ConnectionNum int      `json:"connectionNum"`
	RequestNum    int      `json:"requestNum"`
	MaxTime       float64  `json:"maxTime"`
}

func init() {
	log.Formatter = new(logrus.TextFormatter)
	log.Level = logrus.DebugLevel
}

func main() {
	defer func() {
		err := recover()
		if err != nil {
			log.WithFields(logrus.Fields{
				"异常": err,
			}).Fatal("程序异常！")
		}
	}()
	log.Info("解析服务器列表！")
	fileBytes, err := ioutil.ReadFile("./servers.json")
	if err != nil {
		log.Error("read file error:", err)
	}

	json.Unmarshal(fileBytes, &SS)
	log.Info(SS)

	servers := TestServerSpeed(SS.Servers)
	sort.Sort(BySpeed(servers))

	log.Info(servers)

	toGUIServer(servers)

}

//测试服务器速度
func TestServerSpeed(servers []Server) []Server {
	result := []Server{}
	serverMap := make(map[string]Server)
	testCh := make(chan int, 4)
	wg := sync.WaitGroup{}
	for _, server := range servers {
		if _, ok := serverMap[server.Server+strconv.Itoa(server.ServerPort)]; ok {
			continue
		}
		serverMap[server.Server+strconv.Itoa(server.ServerPort)] = server
		wg.Add(1)
		testCh <- 1
		go func(s Server) {
			defer func() {
				if err := recover(); err != nil {
					log.WithFields(logrus.Fields{
						"异常": err,
					}).Error("速度测试异常！")
				}
				<-testCh
				wg.Done()
			}()
			s.Speed = TestSpeed(s.Server, s.Password, s.Method, strconv.Itoa(s.ServerPort), SS.TestURL, SS.ConnectionNum, SS.RequestNum)
			if s.Speed == 0 {
				s.Speed = float64(5678)
			}
			result = append(result, s)
		}(server)
	}
	wg.Wait()
	return result
}

type BySpeed []Server

func (a BySpeed) Len() int           { return len(a) }
func (a BySpeed) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a BySpeed) Less(i, j int) bool { return a[i].Speed < a[j].Speed }

func toGUIServer(servers []Server) {
	str := []string{}
	for _, server := range servers {
		if server.Speed <= SS.MaxTime {
			proxy := `{"server" : "` + server.Server
			proxy = proxy + `",	"server_port" :` + strconv.Itoa(server.ServerPort)
			proxy = proxy + `,	"password" : "` + server.Password
			proxy = proxy + `",	"method" : "` + server.Method
			proxy = proxy + `",	"remarks" : "` + server.Remarks + `"},`
			str = append(str, proxy)
		}
	}
	all := strings.Join(str, "\r\n")
	os.MkdirAll(path.Dir(SS.OutFile), 0775)
	err := ioutil.WriteFile(SS.OutFile, []byte(all), 0644)
	if err != nil {
		log.Error("write file err:", err)
		panic(err)
	}
}
