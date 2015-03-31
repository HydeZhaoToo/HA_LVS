package main

import (
	"errors"
	"fmt"
	"github.com/Unknwon/goconfig"
	"net/smtp"
	"strings"
	"time"
)

//邮件发送结构
type mailini struct {
	user        string
	passwd      string
	smtpaddress string
	maillist    string
	smtpport    int
}

//从配置文件获取邮件配置
func newmailini(g *goconfig.ConfigFile) *mailini {
	var err error
	var m mailini
	m.maillist, err = g.GetValue("mail", "receive")
	checkerr(err)
	m.user, err = g.GetValue("mail", "mailuser")
	checkerr(err)
	m.passwd, err = g.GetValue("mail", "mailpasswd")
	checkerr(err)
	m.smtpaddress, err = g.GetValue("mail", "smtpaddress")
	checkerr(err)
	m.smtpport = g.MustInt("mail", "smtpport", 25)
	return &m
}

func sendmail(m *mailini, content string) error {
	//格式化邮件内容
	sub := fmt.Sprintf(fmt.Sprintf("To: %s\r\nFrom: %s<%s>\r\nSubject: %s\r\nContent-Type: text/html; Charset=UTF-8\r\n\r\n%s", m.maillist, "易骆驼运维", m.user, "E骆驼集群报警", content))
	mailList := strings.Split(m.maillist, ",")
	auth := smtp.PlainAuth("", m.user, m.passwd, m.smtpaddress)
	au := fmt.Sprintf("%s:%d", m.smtpaddress, m.smtpport)
	errchan := make(chan error)
	defer close(errchan)
	go func() {
		err := smtp.SendMail(au, auth, m.user, mailList, []byte(sub))
		errchan <- err
	}()
	select {
	case err := <-errchan:
		if err != nil {
			return err
		} else {
			return nil
		}
	case <-time.After(time.Second * 10):
		return errors.New("send mail time out more than 10's")
	}

}
