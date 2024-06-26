package mailer

import (
	"bytes"
	"fmt"
	"html/template"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-mail/mail/v2"
	"github.com/mayura-andrew/email-client/internal/data"
)


type Mailer struct {
	dailer *mail.Dialer
	sender string
}

type EmailStatus struct {
	Sent     bool
	Opened   bool
	SentTime time.Time
}

type EmailData struct {
	Subject   string
	Body      string
	Recipient string
	EmailId int64
	URL string
}

func New(host string, port int, username, password, sender string) Mailer {
	dialer := mail.NewDialer(host, port, username, password)
	dialer.Timeout = 5 * time.Second

	return Mailer{
		dailer: dialer,
		sender: sender,
	}
}

func NewMail(e data.EmailModel, host string, port int, username, password, sender, subject string, recipients []string, body string) (map[string]*EmailStatus, error) {

	d := mail.NewDialer(host, port, username, password)

	emailStatuses := make(map[string]*EmailStatus)

	var statusMutex sync.Mutex

	queue := make(chan string)

	var wg sync.WaitGroup

	email := &data.Email{
		Sender:  sender,
		Body:    body,
		Subject: subject,
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for recipient := range queue {

				tmpl, err := template.ParseFiles("./internal/mailer/email_template.tmpl")
				
				if err != nil {
					log.Println(err)
					continue
				}
				url:= os.Getenv("URL")
				fmt.Println("URL: ", url)

				emailId, err := e.InsertEmail(email, recipient)
				if err != nil {
					log.Println(err)
					return
				}

				data := EmailData{
					Subject:   subject,
					Body:      body,
					Recipient: recipient,
					EmailId: emailId,
					URL: url,
				}

				bodyBuf := new(bytes.Buffer)
				err = tmpl.ExecuteTemplate(bodyBuf, "htmlBody", data)

				if err != nil {
					log.Println(err)				
					continue
				}

				m := mail.NewMessage()
				m.SetHeader("From", sender)
				m.SetHeader("To", recipient)
				m.SetHeader("Subject", subject)
				m.SetBody("text/html", bodyBuf.String()) 

				err = d.DialAndSend(m)

				if err != nil {
					fmt.Println("Failed to send test email to -> " + recipient + ": " + err.Error())
				} else {
					fmt.Println("Sent test email successfully to -> " + recipient)
					statusMutex.Lock()
					emailStatuses[recipient].Sent = true
					statusMutex.Unlock()
					err := e.UpdateEmailStatus(emailId)
					if err != nil {
						log.Println(err)
						return
					}
				}
			}
		}()
	}

	for _, recipient := range recipients {
		queue <- recipient

		statusMutex.Lock()
		emailStatuses[recipient] = &EmailStatus{
			Sent:     false,
			Opened:   false,
			SentTime: time.Now(),
		}

		statusMutex.Unlock()
	}
	close(queue)

	wg.Wait()

	for recipient, status := range emailStatuses {
		log.Printf("Email to %s: sent=%v, opened=%v, sentTime=%v", recipient, status.Sent, status.Opened, status.SentTime)
	}

	return emailStatuses, nil
}

func UpdateEmailTracking(e data.EmailModel, emailid int64) error {
	return e.UpdateEmail(emailid)
}
