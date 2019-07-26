package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"github.com/go-martini/martini"
	"github.com/google/uuid"
	"github.com/jinzhu/gorm"
	"github.com/joho/godotenv"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var (
	db     *gorm.DB
	devEnv = flag.Bool("d", false, "setup dev env var")

	timeout = time.Duration(5 * time.Second)
	client  = http.Client{
		Timeout: timeout,
	}

	OuathScopes       = []string{"User.Read", "offline_access", "User.ReadBasic.All"}
	ClientIdConst     = getenv("CLIENT_ID")
	TenantConst       = getenv("TENANT")
	ClientSecretConst = getenv("CLIENT_SECRET")
	BaseUrl           = getenv("BASE_URL")
	RedirectPath      = getenv("REDIRECT_PATH")
	dialect           = "sqlite3"
	connStr           = "./authData"
	authority         = Authority{"login.microsoftonline.com", os.Getenv("TENANT")}
)

func getenv(name string) string {
	_, err := os.Stat(".env")
	if err == nil {
		godotenv.Load()
	}
	v := os.Getenv(name)
	if v == "" {
		panic("Missing required environment variable " + name)
	}
	return v
}

func getMeHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("------------------------------------------------------")
	fmt.Println("Start get me")
	fmt.Println("------------------------------------------------------")
	token := r.Header.Get("Authorization")

	user := FindUserByPubToken(token)
	if (user == User{}) {
		http.Error(w, "Record not found", http.StatusNotFound)
		authUrl := fmt.Sprint(BaseUrl, "/auth")
		http.Redirect(w, r, authUrl, http.StatusNotFound)
	}
	meResponse := getMeRequest(user.AccessToken)
	defer meResponse.Body.Close()

	if meResponse.StatusCode != 200 {
		retryWithRefresh(&user)
		meResponse = getMeRequest(user.AccessToken)

		if meResponse.StatusCode != 200 {
			authUrl := fmt.Sprint(BaseUrl, "/auth")
			http.Redirect(w, r, authUrl, http.StatusNotFound)
		}
	}

	meBytes, err := ioutil.ReadAll(meResponse.Body)
	if err != nil {
		panic(fmt.Errorf("ERROR: %s", err))
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(meBytes)
}

func retryWithRefresh(user *User) {
	fmt.Println("------------------------------------------------------")
	fmt.Println("Trying to refresh token")
	fmt.Println("------------------------------------------------------")
	params := url.Values{}

	params.Add("grant_type", "refresh_token")
	params.Add("refresh_token", user.RefreshToken)
	params.Add("client_id", ClientIdConst)
	params.Add("client_secret", ClientSecretConst)
	params.Add("resource", "https://graph.microsoft.com")

	urlBytes := []byte(strings.TrimSpace(params.Encode()))

	request, err := http.NewRequest("POST", fmt.Sprint(authority), bytes.NewReader(urlBytes))
	if err != nil {
		panic(fmt.Errorf("ERROR: %s", err))
	}

	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	request.Header.Set("client-request-id", fmt.Sprint(uuid.Must(uuid.NewRandom())))
	request.Header.Set("client-return-client-request-id", "true")

	response, err := client.Do(request)
	if response.StatusCode != 200 {
		fmt.Errorf("ERROR: %s", "Can not refresh token, try to auth again")
		return
	}
	defer response.Body.Close()

	var refreshTokenResponse refreshTokenResponse
	meBytes, err := ioutil.ReadAll(response.Body)
	if err != nil {
		fmt.Errorf("ERROR: %s", err)

	}

	err = json.Unmarshal(meBytes, &refreshTokenResponse)
	if err != nil {
		fmt.Errorf("ERROR: %s", err)

	}

	RefreshToken(user, refreshTokenResponse)

}

func getPhotoHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("------------------------------------------------------")
	fmt.Println("Start get photo")
	fmt.Println("------------------------------------------------------")
	token := r.Header.Get("Authorization")

	user := FindUserByPubToken(token)
	if (user == User{}) {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	tokenStr := fmt.Sprint("Bearer ", user.AccessToken)

	picRequest, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me/photo/$value", nil)
	picRequest.Header.Set("Authorization", tokenStr)
	picResponse, err := client.Do(picRequest)
	if picResponse.StatusCode != 200 {
		fmt.Println("Something went wrong, accessing user picture")
		return
	}
	if err != nil {
		fmt.Errorf("ERROR: %s", err)
		return
	}
	pictureBinary, _ := ioutil.ReadAll(picResponse.Body)
	if pictureBinary == nil {
		w.Write([]byte{})
	}

	w.Write(pictureBinary)
}

func main() {
	flag.Parse()

	db = InitDB()
	defer db.Close()

	m := martini.Classic()

	m.Get("/get_me", getMeHandler)
	m.Get("/get_user_photo", getPhotoHandler)
	m.Post("/auth_with_temporary_token", authWithTempTokenHandler)
	m.Get("/auth", oauthHandler)
	m.Get("/auth_url", oauthUrlHandler)
	m.Get(RedirectPath, aadAuthHandler)
	m.Run()
}
