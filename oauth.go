package main

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io/ioutil"
	"net/http"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/microsoft"
)

var (
	xOauth2Config = oauth2.Config{
		ClientID:     ClientIdConst,
		ClientSecret: ClientSecretConst,
		RedirectURL:  fmt.Sprint(BaseUrl, RedirectPath),
		Endpoint:     microsoft.AzureADEndpoint(TenantConst),
		Scopes:       OuathScopes,
	}
)

func oauthUrlHandler(w http.ResponseWriter, r *http.Request) {
	authUrl := fmt.Sprint(BaseUrl, "/auth")
	fmt.Fprintf(w, authUrl)
}

// Auth handler which will redirect to AAD
func oauthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("------------------------------------------------------")
	fmt.Println("Start authentication")
	fmt.Println("------------------------------------------------------")
	state := randToken(48)
	authorizationURL := xOauth2Config.AuthCodeURL(state)
	http.Redirect(w, r, authorizationURL, 301)
}

// process the redirection from AAD
func aadAuthHandler(w http.ResponseWriter, r *http.Request) {
	fmt.Println("------------------------------------------------------")
	fmt.Println("Start handling callback from Azure")
	fmt.Println("------------------------------------------------------")
	authorizationCode := r.URL.Query().Get("code")

	ck, err := r.Cookie("state")
	if err == nil && (r.URL.Query().Get("state") != ck.Value) {
		fmt.Fprintf(w, "Error: State is not the same")
	}
	oAuthToken, err := xOauth2Config.Exchange(context.Background(), authorizationCode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	meResponse := getMeRequest(oAuthToken.AccessToken)
	defer meResponse.Body.Close()

	var azureUserInfo AzureUserInfo
	meBytes, err := ioutil.ReadAll(meResponse.Body)
	handleError(err)

	err = json.Unmarshal(meBytes, &azureUserInfo)
	handleError(err)

	token := OToken{Token: oAuthToken, PublicToken: "", TemporaryToken: fmt.Sprint(uuid.New())}

	user := FindOrCreateUser(&token, &azureUserInfo)
	if (User{} == user) {
		authUrl := fmt.Sprint(BaseUrl, "/auth")
		http.Redirect(w, r, authUrl, http.StatusNotFound)
	}

	tempTokenURL := generateTempTokenUrl(token.TemporaryToken)
	http.Redirect(w, r, tempTokenURL, http.StatusFound)
}

func getMeRequest(token string) *http.Response {
	meRequest, err := http.NewRequest("GET", "https://graph.microsoft.com/v1.0/me", nil)
	if err != nil {
		panic(fmt.Errorf("ERROR: %s", err))
	}

	tokenStr := fmt.Sprint("Bearer ", token)
	meRequest.Header.Set("Authorization", tokenStr)

	meResponse, err := client.Do(meRequest)

	if err != nil {
		fmt.Errorf("ERROR: %s", err)
	}
	fmt.Println("------------------------------------------------------")
	fmt.Println("Get me request successfully")
	fmt.Println("------------------------------------------------------")

	return meResponse
}

func authWithTempTokenHandler(w http.ResponseWriter, r *http.Request) {
	keys := r.URL.Query()

	temporaryToken := keys.Get("temporary_token")

	user := FindUserByTempToken(temporaryToken)
	if (user == User{}) {
		http.Error(w, "Record not found", http.StatusNotFound)
		return
	}

	var oAuthToken oauth2.Token
	token := OToken{Token: &oAuthToken, PublicToken: fmt.Sprint(uuid.New()), TemporaryToken: ""}
	user.UpdateToken(&token)

	fmt.Fprintf(w, user.ClientPublicToken)
}
