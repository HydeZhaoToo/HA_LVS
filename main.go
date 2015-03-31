package main

import (
	"encoding/binary"
	"errors"
	"flag"
	"fmt"
	"github.com/Unknwon/goconfig"
	_ "github.com/go-sql-driver/mysql"
	"github.com/go-xorm/xorm"
	"log"
	"net/http"
	"time"
)

/*
lvs条目信息
*/
type lvs_info struct {
	port   string //虚拟端口
	vip    string //虚拟IP
	realip string //真实IP
	after  bool   //状态 false表示在集群 true表示不在集群
	Type   string //服务类型
}

var (
	/*
		http检测客户端 超时设置5秒
	*/
	curl_client *http.Client = &http.Client{
		Timeout: 10 * time.Second,
	}
	configfile   string
	mysql_user   string = "root"
	mysql_passwd string = "123456"
	/*
		lvs条目信息
	*/
	dest_lvs map[string]*lvs_info = map[string]*lvs_info{
		"realweb1": &lvs_info{
			port:   "80",
			vip:    "192.168.1.165",
			realip: "192.168.1.171",
			after:  false,
			Type:   "http",
		},
		"realweb2": &lvs_info{
			port:   "80",
			vip:    "192.168.1.165",
			realip: "192.168.1.172",
			after:  false,
			Type:   "http",
		},
		"serverweb1": &lvs_info{
			port:   "80",
			vip:    "192.168.1.166",
			realip: "192.168.1.186",
			after:  false,
			Type:   "http",
		},
		"serverweb2": &lvs_info{
			port:   "80",
			vip:    "192.168.1.166",
			realip: "192.168.1.187",
			after:  false,
			Type:   "http",
		},
		"sourceweb1": &lvs_info{
			port:   "80",
			vip:    "192.168.1.167",
			realip: "192.168.1.176",
			after:  false,
			Type:   "http",
		},
		"sourceweb2": &lvs_info{
			port:   "80",
			vip:    "192.168.1.167",
			realip: "192.168.1.177",
			after:  false,
			Type:   "http",
		},
		"mysqlslave1": &lvs_info{
			port:   "3306",
			vip:    "192.168.1.165",
			realip: "192.168.1.191",
			after:  false,
			Type:   "mysql",
		},
		"mysqlslave2": &lvs_info{
			port:   "3306",
			vip:    "192.168.1.165",
			realip: "192.168.1.192",
			after:  false,
			Type:   "mysql",
		},
	}
)

func init() {
	flag.StringVar(&configfile, "c", "", "配置文件路径")
	flag.Parse()
	/*
		初始化 日志格式
	*/
	log.SetFlags(log.Ltime | log.Llongfile)
}

func check_slave(dest string) error {
	//生成服务器连接
	engine, err := xorm.NewEngine("mysql", fmt.Sprintf("root:mysqlpasswd@tcp(%s:3306)/?charset=utf8", dest))
	if err != nil {
		return err
	}
	defer engine.Close()
	//查询从库状态
	slave_result, err := engine.Query("show slave status")
	if err != nil {
		return err
	}
	if string(slave_result[0]["Slave_IO_Running"]) != "Yes" {
		return errors.New(fmt.Sprintf("mysql:%s Last_IO_Error:%s", dest, string(slave_result[0]["Last_IO_Error"])))
	}
	if string(slave_result[0]["Slave_SQL_Running"]) != "Yes" {
		return errors.New(fmt.Sprintf("mysql:%s Last_IO_Error:%s", dest, string(slave_result[0]["Last_SQL_Error"])))
	}
	behind_time, _ := binary.Uvarint(slave_result[0]["Seconds_Behind_Master"])
	if behind_time > 60 {
		return errors.New(fmt.Sprintf("behind master %d seconds ", behind_time))
	}
	return nil
}

func check_http(dest string) error {
	resp, err := curl_client.Get(fmt.Sprintf("http://%s", dest))

	//http.Get(fmt.Sprintf("http://%s", dest))
	//resp.Body.Close()
	if err != nil {
		return err
	}
	if resp.StatusCode == 200 || resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 304 || resp.StatusCode == 403 {
		return nil
	}
	return errors.New(fmt.Sprintf("Http StatusCode: %d", resp.StatusCode))
}

func checkerr(err error) {
	if err != nil {
		log.Fatalln(err)
	}
}

func main() {
	//生成打点器
	check_ticker := time.NewTicker(time.Second * 20)
	//读取配置
	g, err := goconfig.LoadConfigFile(configfile)
	checkerr(err)
	//初始化邮件配置
	mailini_tmp := newmailini(g)
	checkerr(err)
	for _ = range check_ticker.C {
		for j, v := range dest_lvs {
			//判断检测服务类型
			if v.Type == "http" {
				//检测服务
				if err := check_http(v.realip); err != nil {
					//服务是否在线
					if !v.after {
						go func() {
							err := sendmail(mailini_tmp, fmt.Sprintf("HostName:%s<br>Ip:%s<br>ErrorL%s", j, v.realip, err.Error()))
							if err != nil {
								log.Println(err)
							}
							conn, err := ssh_dial(v.vip+":22", mysql_user, mysql_passwd)
							if err != nil {
								log.Println(err)
							}
							err = ssh_exec(conn, v)
							if err != nil {
								log.Println(err)
							}
						}()
					}
				} else {
					if v.after {
						//检测恢复在线
						go func() {
							conn, err := ssh_dial(v.vip+":22", mysql_user, mysql_passwd)
							if err != nil {
								log.Println(err)
							}
							err = ssh_exec(conn, v)
							if err != nil {
								log.Println(err)
							}
						}()
					}
				}
			}
			if v.Type == "mysql" {
				if err := check_slave(v.realip); err != nil {
					if !v.after {
						go func() {
							err = sendmail(mailini_tmp, fmt.Sprintf("HostName:%s<br>Ip:%s<br>Error:%s", j, v.realip, err.Error()))
							if err != nil {
								log.Println(err)
							}
							conn, err := ssh_dial(v.vip+":22", mysql_user, mysql_passwd)
							if err != nil {
								log.Println(err)
							}
							err = ssh_exec(conn, v)
							if err != nil {
								log.Println(err)
							}
						}()
					}
				} else {
					if v.after {
						go func() {
							conn, err := ssh_dial(v.vip+":22", mysql_user, mysql_passwd)
							if err != nil {
								log.Println(err)
							}
							err = ssh_exec(conn, v)
							if err != nil {
								log.Println(err)
							}
						}()
					}
				}
			}
		}
	}

}
