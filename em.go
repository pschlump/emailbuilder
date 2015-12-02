/*

Package em is a wrapper around mailbuilder that provides a different
syntax for using the mail builder.

// (C) Philip Schlump, 2014-2015.
// (C) Rod Brown, 2014.

Use a JSON file that looks like

-- cut --
{
	 "Username":"yourname@gmail.com"
	,"Password":"yourpassword"
	,"EmailServer":"smtp.gmail.com"
	,"Port":587
}
-- cut --

Put the file in $HOME/.email/email-config.json

*/
package em

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io/ioutil"
	"mime"
	"net/smtp"
	"path/filepath"
	"strconv"

	"github.com/pschlump/json"       //	"encoding/json"
	"github.com/zerobfd/mailbuilder" // "../mailbuilder"

	ms "github.com/pschlump/templatestrings" // "../ms"
)

const (
	Version = "Version: 1.0.0"
)

// ---------------------------------------------------------------------------------------------------------------------
type EmailUser struct {
	Username    string // Something like you@yourdomain.com
	Password    string // Your paassword like password123
	EmailServer string // smtp.gmail.com
	Port        int    // 587 for example
}

type EM struct {
	EmailCfgFn  string
	EmailConfig EmailUser
	SmtpAuth    smtp.Auth
	Message     *mailbuilder.Message
	Alt         *mailbuilder.MultiPart
	Mixed       *mailbuilder.MultiPart
	Err         error

	altSetup      bool
	lineMaxLength int
	printErrors   bool
}

// ---------------------------------------------------------------------------------------------------------------------
func (this *EM) initEM() {
	this.SmtpAuth = smtp.PlainAuth("",
		this.EmailConfig.Username,
		this.EmailConfig.Password,
		this.EmailConfig.EmailServer)
	this.Message = mailbuilder.NewMessage()
}

func (this *EM) readEmailConfig(fn string) (rv EmailUser, err error) {
	data, err := ioutil.ReadFile(fn)
	if err != nil {
		e := fmt.Sprintf("Error(12023): File (%s) missing or unreadable error: %v\n", fn, err)
		err = errors.New(e)
	} else {
		err := json.Unmarshal(data, &rv)
		if err != nil {
			e := fmt.Sprintf("Error(12002): Invalid format - %v\n", err)
			err = errors.New(e)
		}
	}
	return
}

func NewEmFile(fn string, pe bool) *EM {
	var err error
	x := EM{EmailCfgFn: fn, altSetup: false, lineMaxLength: 500, printErrors: pe, Err: nil}
	if fn[0:1] == "/" {
		x.EmailConfig, err = x.readEmailConfig(x.EmailCfgFn)
	} else if fn[0:2] == "~/" {
		x.EmailConfig, err = x.readEmailConfig(ms.HomeDir() + "/" + x.EmailCfgFn[2:])
	} else {
		x.EmailConfig, err = x.readEmailConfig("./" + x.EmailCfgFn)
	}
	if err != nil {
		x.Err = err
		if x.printErrors {
			fmt.Printf("%v\n", x.Err)
		}
	}
	x.initEM()
	return &x
}

func NewEm(ec EmailUser) *EM {
	x := EM{EmailCfgFn: "", altSetup: false, lineMaxLength: 500, printErrors: true, Err: nil}
	x.EmailConfig = ec
	x.initEM()
	return &x
}

// The default is 500 wich is a good length for liens.  If you need ti to be shorter then
// this schould be called afer NewEmFile or NewEm
func (this *EM) SetMaxLineLength(n int) {
	this.lineMaxLength = n
}

// em will print out error messages by default.  If you need to turn this off so that error
// messages are only returnd then pass 'false'.  Should be called after NewEmFile or
// NewEm
func (this *EM) SetPrintErrors(b bool) {
	this.printErrors = b
}

// Set the destination address, may be called more than onece.
func (this *EM) To(addr string, name string) *EM {
	this.Message.AddTo(mailbuilder.NewAddress(addr, name))
	return this
}

// Set the CC: destination address, may be called more than onece.
func (this *EM) Cc(addr string, name string) *EM {
	this.Message.AddCc(mailbuilder.NewAddress(addr, name))
	return this
}

// Set the BCC: destination address, may be called more than onece.
func (this *EM) Bcc(addr string, name string) *EM {
	this.Message.AddBcc(mailbuilder.NewAddress(addr, name))
	return this
}

// Set the source of the message
func (this *EM) From(addr string, name string) *EM {
	this.Message.From = mailbuilder.NewAddress(addr, name)
	return this
}

// Set the Subject for the email.
func (this *EM) Subject(s string) *EM {
	this.Message.Subject = s
	return this
}

func (this *EM) doAltSetup() {
	if !this.altSetup {
		this.altSetup = true
		this.Alt = mailbuilder.NewMultiPart("multipart/alternative")
		this.Mixed = mailbuilder.NewMultiPart("multipart/mixed")
		this.Mixed.AddPart(this.Alt)
	}
}

// Send a Text body with the email.
func (this *EM) TextBody(s string) *EM {
	this.doAltSetup()

	text := mailbuilder.NewSimplePart()
	// add content/headers to html and text
	//text.AddHeader("Content-Type", "text/plain; charset=utf8")
	//text.AddHeader("Content-Transfer-Encoding", "quoted-printable")
	text.AddHeader("Content-Type", "text/plain; charset=us-ascii")
	text.Content = s
	this.Alt.AddPart(text)
	return this
}

// Send a HTML body with the email.
func (this *EM) HtmlBody(s string) *EM {
	this.doAltSetup()

	html := mailbuilder.NewSimplePart()
	// add content/headers to html and text
	//	html.AddHeader("Content-Type", "text/html; charset=utf8")
	//	html.AddHeader("Content-Transfer-Encoding", "quoted-printable")
	html.AddHeader("Content-Type", "text/html; charset=us-ascii")
	html.Content = s
	this.Alt.AddPart(html)
	return this
}

// Attach a file to the email - may be a relative path.  The file name that is sent in
// the email will be the base file name.
func (this *EM) Attach(fn string) *EM {
	this.doAltSetup()

	bfn := filepath.Base(fn)
	ext := filepath.Ext(fn)
	ct := mime.TypeByExtension(ext)

	attch := mailbuilder.NewSimplePart()
	attch.AddHeader("Content-Type", ct)
	attch.AddHeader("Content-Transfer-Encoding", "base64")
	attch.AddHeader("Content-Disposition", fmt.Sprintf(`attachment; filename=%q`, bfn))

	//read and encode attachment
	content, _ := ioutil.ReadFile(fn)
	encoded := base64.StdEncoding.EncodeToString(content)
	//split the encoded file in lines (doesn't matter, but low enough not to hit a max limit)
	nbrLines := len(encoded) / this.lineMaxLength
	var buf bytes.Buffer
	for i := 0; i < nbrLines; i++ {
		buf.WriteString(encoded[i*this.lineMaxLength:(i+1)*this.lineMaxLength] + "\n")
	}
	buf.WriteString(encoded[nbrLines*this.lineMaxLength:])
	attch.Content = buf.String()

	this.Mixed.AddPart(attch)
	return this
}

// Last call.  This sends the message.
func (this *EM) SendIt() (err error) {

	if !this.altSetup {
		err = errors.New("Error(12022): Can not send an email without a body or attachments.")
		return
	}

	this.Message.SetBody(this.Mixed)

	err = smtp.SendMail(this.EmailConfig.EmailServer+":"+strconv.Itoa(this.EmailConfig.Port),
		this.SmtpAuth,
		this.Message.From.Email,
		this.Message.Recipients(),
		this.Message.Bytes())

	if err != nil {
		e := fmt.Sprintf("Error(12021): SMTP Send Error: %v", err)
		if this.printErrors {
			fmt.Printf("%s\n", e)
		}
		err = errors.New(e)
		this.Err = err
	}

	this.Message = mailbuilder.NewMessage()

	return
}

/* vim: set noai ts=4 sw=4: */
