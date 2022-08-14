package main

import (
	"fmt"
	"go-subscription/data"
	"html/template"
	"net/http"
)

func (app *Config) HomePage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "home.page.gohtml", nil)
}

func (app *Config) LoginPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "login.page.gohtml", nil)
}

func (app *Config) PostLoginPage(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.RenewToken(r.Context())
	err := r.ParseForm()
	if err != nil {
		app.ErrorLog.Println(err)
	}
	email := r.Form.Get("email")
	password := r.Form.Get("password")
	user, err := app.Models.User.GetByEmail(email)

	if err != nil {
		app.Session.Put(r.Context(), "error", "Invalid credentials")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	validPassword, err := user.PasswordMatches(password)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Invalid credentials")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	if !validPassword {
		msg := Message{
			To:      email,
			Subject: "Failed login",
			Data:    "Failed login attempt",
		}
		app.sendEmail(msg)
		app.Session.Put(r.Context(), "error", "Invalid credentials")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}
	app.Session.Put(r.Context(), "userID", user.ID)
	app.Session.Put(r.Context(), "user", user)

	app.Session.Put(r.Context(), "flash", "Successful login!")

	http.Redirect(w, r, "/login", http.StatusSeeOther)

}

func (app *Config) Logout(w http.ResponseWriter, r *http.Request) {
	_ = app.Session.Destroy(r.Context())
	_ = app.Session.RenewToken(r.Context())

	http.Redirect(w, r, "/login", http.StatusSeeOther)

}

func (app *Config) RegisterPage(w http.ResponseWriter, r *http.Request) {
	app.render(w, r, "register.page.gohtml", nil)
}

func (app *Config) ChooseSubscription(w http.ResponseWriter, r *http.Request) {
	if !app.Session.Exists(r.Context(), "userID") {
		app.Session.Put(r.Context(), "warning", "You must login to see this page.")
		http.Redirect(w, r, "/login", http.StatusTemporaryRedirect)
	}
	plans, err := app.Models.Plan.GetAll()
	if err != nil {
		app.ErrorLog.Println("Cant get plans")
		return
	}
	dataMap := make(map[string]any)
	dataMap["plans"] = plans

	app.render(w, r, "plans.page.gohtml", &TemplateData{
		Data: dataMap,
	})
}

func (app *Config) PostRegisterPage(w http.ResponseWriter, r *http.Request) {
	// create user
	_ = app.Session.RenewToken(r.Context())
	err := r.ParseForm()
	if err != nil {
		app.ErrorLog.Println(err)
	}
	u := data.User{
		Email:     r.Form.Get("email"),
		Password:  r.Form.Get("password"),
		FirstName: r.Form.Get("first-name"),
		LastName:  r.Form.Get("last-name"),
		Active:    0,
		IsAdmin:   0,
	}

	fmt.Println(u)
	// check if user exists
	//user, err := app.Models.User.GetByEmail(email)

	_, err = u.Insert(u)
	if err != nil {
		app.Session.Put(r.Context(), "error", "Unable to create user.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		app.ErrorLog.Println(err)
		return
	}
	url := fmt.Sprintf("http://localhost/activate?email=%s", u.Email)
	signedURL := GenerateTokenFromString(url)
	app.InfoLog.Println(signedURL)

	msg := Message{
		From:     "subscribe@example.com",
		FromName: "Activate",
		To:       u.Email,
		Subject:  "Activate your account",
		Data:     template.HTML(signedURL),
		Template: "confirmation-email",
	}

	app.sendEmail(msg)

	app.Session.Put(r.Context(), "flash", "Confirmation email sent.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (app *Config) ActivateAcount(w http.ResponseWriter, r *http.Request) {
	// validate url
	url := r.RequestURI
	testURL := fmt.Sprintf("http://localhost%s", url)
	fmt.Println("test ", testURL)

	okay := VerifyToken(testURL)
	if !okay {
		app.Session.Put(r.Context(), "error", "Invalid token")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		return
	}

	u, err := app.Models.User.GetByEmail(r.URL.Query().Get("email"))
	if err != nil {
		app.Session.Put(r.Context(), "error", "No user found")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		app.ErrorLog.Println(err)
		return
	}
	u.Active = 1
	err = u.Update()
	if err != nil {
		app.Session.Put(r.Context(), "error", "Unable to update user.")
		http.Redirect(w, r, "/register", http.StatusSeeOther)
		app.ErrorLog.Println(err)
		return
	}

	app.Session.Put(r.Context(), "flash", "Account activated. You can now login")
	http.Redirect(w, r, "/login", http.StatusSeeOther)

	// msg := Message{
	// 	To:       u.Email,
	// 	FromName: u.FirstName,
	// 	Subject:  "Successfuly subscrived",
	// 	Data:     "User is now active",
	// 	Template: "subscribed-email",
	// }

	// app.sendEmail(msg)

	// generate invoice

	// send an email with attachments
}
