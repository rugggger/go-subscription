package main

func (app *Config) sendEmail(msg Message) {
	app.Mailer.Wait.Add(1)
	app.Mailer.MailerChan <- msg
}
