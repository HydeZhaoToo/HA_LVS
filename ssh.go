package main

import (
	"code.google.com/p/go.crypto/ssh"
	"fmt"
)

//初始化连接
func ssh_dial(network, user, passwd string) (*ssh.Client, error) {
	config := &ssh.ClientConfig{
		User: user,
		Auth: []ssh.AuthMethod{
			ssh.Password(passwd),
		},
	}
	return ssh.Dial("tcp", network, config)
}

//执行语句
func ssh_exec(conn *ssh.Client, li *lvs_info) error {
	session, err := conn.NewSession()
	if err != nil {
		fmt.Println("newsession" + err.Error())
		return err
	}
	defer session.Close()

	if err := session.Run(format_command(li)); err != nil { //如果失败返回失败信息
		fmt.Println("run " + err.Error())
		return err
	} else { //如果成功修改lvs_info状态
		li.after = !li.after
		return nil
	}
}

func format_command(li *lvs_info) string {
	if li.after {
		fmt.Println(fmt.Sprintf("/sbin/ipvsadm -a -t %s:%s -r %s:%s -g", li.vip, li.port, li.realip, li.port))
		return fmt.Sprintf("/sbin/ipvsadm -a -t %s:%s -r %s:%s -g", li.vip, li.port, li.realip, li.port)
	} else {
		fmt.Println(fmt.Sprintf("/sbin/ipvsadm -d -t %s:%s -r %s:%s", li.vip, li.port, li.realip, li.port))
		return fmt.Sprintf("/sbin/ipvsadm -d -t %s:%s -r %s:%s", li.vip, li.port, li.realip, li.port)
	}
}
