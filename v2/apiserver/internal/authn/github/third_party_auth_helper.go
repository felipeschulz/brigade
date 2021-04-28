package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"strings"

	"github.com/brigadecore/brigade/v2/apiserver/internal/authn"
	"github.com/google/go-github/v33/github"
	"github.com/pkg/errors"
	"golang.org/x/oauth2"
)

// ThirdPartyAuthHelperConfig encapsulates configuration for the GitHub-based
// implementation of the authn.ThirdPartyAuthHelper interface.
type ThirdPartyAuthHelperConfig struct {
	// ClientID is the client ID for a GitHub OAuth App.
	ClientID string
	// ClientSecret is the client secret for a GitHub OAuth App.
	ClientSecret string
}

type thirdPartyAuthHelper struct {
	config ThirdPartyAuthHelperConfig
	// The following behaviors are overridable for test purposes
	getOAuth2AccessTokenFn func(oauth2State, oauth2Code string) (string, error)
	getUserIdentityFn      func(
		ctx context.Context,
		accessToken string,
	) (authn.ThirdPartyIdentity, error)
}

func NewThirdPartyAuthHelper(
	config ThirdPartyAuthHelperConfig,
) authn.ThirdPartyAuthHelper {
	t := &thirdPartyAuthHelper{
		config: config,
	}
	t.getOAuth2AccessTokenFn = t.getOAuth2AccessToken
	t.getUserIdentityFn = t.getUserIdentity
	return t
}

func (t *thirdPartyAuthHelper) AuthURL(oauth2State string) string {
	return fmt.Sprintf(
		"https://github.com/login/oauth/authorize?client_id=%s&state=%s",
		url.QueryEscape(t.config.ClientID),
		url.QueryEscape(oauth2State),
	)
}

func (t *thirdPartyAuthHelper) Exchange(
	ctx context.Context,
	oauth2State string,
	oauth2Code string,
) (authn.ThirdPartyIdentity, error) {
	accessToken, err := t.getOAuth2AccessTokenFn(oauth2State, oauth2Code)
	if err != nil {
		return authn.ThirdPartyIdentity{},
			errors.Wrap(err, "error exchanging code for OAuth2 access token")
	}
	identity, err := t.getUserIdentityFn(ctx, accessToken)
	return identity,
		errors.Wrap(err, "error retrieving user identity from GitHub")
}

func (t *thirdPartyAuthHelper) getOAuth2AccessToken(
	oauth2State string,
	oauth2Code string,
) (string, error) {
	form := url.Values{}
	form.Add("client_id", t.config.ClientID)
	form.Add("client_secret", t.config.ClientSecret)
	form.Add("state", oauth2State)
	form.Add("code", oauth2Code)
	req, err := http.NewRequest(
		http.MethodPost,
		"https://github.com/login/oauth/access_token",
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return "", err
	}
	req.Header.Add("Accept", "application/json")
	var resp *http.Response
	if resp, err = http.DefaultClient.Do(req); err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var bodyBytes []byte
	if bodyBytes, err = ioutil.ReadAll(resp.Body); err != nil {
		return "", nil
	}
	bodyStruct := struct {
		AccessToken string `json:"access_token"`
	}{}
	err = json.Unmarshal(bodyBytes, &bodyStruct)
	return bodyStruct.AccessToken, err
}

func (t *thirdPartyAuthHelper) getUserIdentity(
	ctx context.Context,
	accessToken string,
) (authn.ThirdPartyIdentity, error) {
	githubClient := github.NewClient(
		oauth2.NewClient(
			ctx,
			oauth2.StaticTokenSource(
				&oauth2.Token{
					TokenType:   "Bearer",
					AccessToken: accessToken,
				},
			),
		),
	)
	gitHubUser, _, err := githubClient.Users.Get(ctx, "")
	if err != nil {
		return authn.ThirdPartyIdentity{}, err
	}
	return authn.ThirdPartyIdentity{
		ID:   gitHubUser.GetLogin(),
		Name: gitHubUser.GetName(),
	}, nil
}
