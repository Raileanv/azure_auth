package main

import (
	"github.com/jinzhu/gorm"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
	"golang.org/x/oauth2"
	"time"
)

type Model struct {
	ID        int `gorm:"AUTO_INCREMENT;PRIMARY_KEY"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type User struct {
	gorm.Model
	AzureId           string
	Name              string
	AccessToken       string
	RefreshToken      string
	ClientPublicToken string
	TemporaryToken    string
}

type AzureUserInfo struct {
	OdataContext      string        `json:"@odata.context"`
	BusinessPhones    []interface{} `json:"businessPhones"`
	DisplayName       string        `json:"displayName"`
	GivenName         string        `json:"givenName"`
	JobTitle          string        `json:"jobTitle"`
	Mail              string        `json:"mail"`
	MobilePhone       interface{}   `json:"mobilePhone"`
	OfficeLocation    string        `json:"officeLocation"`
	PreferredLanguage interface{}   `json:"preferredLanguage"`
	Surname           string        `json:"surname"`
	UserPrincipalName string        `json:"userPrincipalName"`
	ID                string        `json:"id"`
}

type refreshTokenResponse struct {
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope"`
	ExpiresIn    string `json:"expires_in"`
	ExtExpiresIn string `json:"ext_expires_in"`
	ExpiresOn    string `json:"expires_on"`
	NotBefore    string `json:"not_before"`
	Resource     string `json:"resource"`
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

type OToken struct {
	*oauth2.Token
	TemporaryToken string
	PublicToken    string
}

func InitDB() *gorm.DB {
	if !*devEnv {
		//connStr = fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=disable",
		//	getenv("DB_HOST"), getenv("DB_PORT"), getenv("DB_USER"), getenv("DB_PASSWORD"), getenv("DB_NAME"))
		connStr = getenv("DATABASE_URL")
		dialect = "postgres"

	}

	db, err := gorm.Open(dialect, connStr)
	if err != nil {
		panic(err)
	}
	if db == nil {
		panic("db nil")
	}
	db.AutoMigrate(&User{})
	return db
}

func FindUserByTempToken(token string) (user User) {
	user = User{}
	db.Find(&user, "temporary_token = ?", token)
	return
}
func FindUserByPubToken(token string) (user User) {
	user = User{}
	db.Find(&user, "client_public_token = ?", token)
	return
}

func (user *User) UpdateToken(t *OToken) {
	if t.AccessToken != "" {
		user.AccessToken = t.AccessToken
	}
	user.TemporaryToken = t.TemporaryToken
	user.ClientPublicToken = t.PublicToken

	db.Save(&user)
}

func FindOrCreateUser(token *OToken, userInfo *AzureUserInfo) User {
	user := User{}
	db.Find(&user, "azure_id = ?", userInfo.ID)
	if (User{} != user) {
		user.UpdateToken(token)
		return user
	}
	user.Create(token, userInfo)

	return user
}

func RefreshToken(user *User, r refreshTokenResponse) {
	user.AccessToken = r.AccessToken
	user.RefreshToken = r.RefreshToken

	db.Save(&user)
}

func (user *User) Create(t *OToken, ui *AzureUserInfo) {
	user.AccessToken = t.AccessToken
	user.TemporaryToken = t.TemporaryToken
	user.ClientPublicToken = t.PublicToken
	user.RefreshToken = t.RefreshToken
	user.Name = ui.DisplayName
	user.AzureId = ui.ID

	db.Create(&user)
}
