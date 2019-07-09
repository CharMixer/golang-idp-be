package main

import (
  "fmt"
  "strings"
  "net/http"
  "net/url"


  "golang.org/x/net/context"
  "golang.org/x/oauth2"
  "golang.org/x/oauth2/clientcredentials"

  oidc "github.com/coreos/go-oidc"
  "github.com/gin-gonic/gin"
  "github.com/atarantini/ginrequestid"


  "golang-idp-be/config"
  "golang-idp-be/gateway/idpbe"
  "golang-idp-be/identities"
)

const app = "idpbe"
const accessTokenKey = "access_token"
const requestIdKey = "RequestId"

func init() {
  config.InitConfigurations()
}

func main() {

  provider, err := oidc.NewProvider(context.Background(), config.Hydra.Url + "/")
  if err != nil {
    fmt.Println(err)
    return
  }

  // Setup the hydra client idpbe is going to use (oauth2 client credentials)
  // NOTE: We store the hydraConfig also as we are going to need it to let idpbe app start the Oauth2 Authorization code flow.
  hydraConfig := &clientcredentials.Config{
    ClientID:     config.IdpBe.ClientId,
    ClientSecret: config.IdpBe.ClientSecret,
    TokenURL:     provider.Endpoint().TokenURL,
    Scopes:       config.IdpBe.RequiredScopes,
    EndpointParams: url.Values{"audience": {"hydra"}},
    AuthStyle: 2, // https://godoc.org/golang.org/x/oauth2#AuthStyle
  }
  //hydraClient := hydra.NewHydraClient(hydraConfig)

  dbIdentities := map[string]*idpbe.Identity{
    "user-1": &idpbe.Identity{
        Id: "user-1",
        Name: "Test",
        Email: "user-1@domain.com",
        Password: "1234",
    },
    "wraix": &idpbe.Identity{
      Id: "wraix",
      Name: "wraix",
      Email: "wraix@domain.com",
      Password: "5678",
    },
  }

  // Setup app state variables. Can be used in handler functions by doing closures see exchangeAuthorizationCodeCallback
  env := &idpbe.IdpBeEnv{
    Provider: provider,
    HydraConfig: hydraConfig,
    //HydraClient: hydraClient, // Will this serialize the request handling?
    Database: dbIdentities,
  }

  r := gin.Default()
  r.Use(ginrequestid.RequestId())

  // ## QTNA - Questions that need answering before granting access to a protected resource
  // 1. Is the user or client authenticated? Answered by the process of obtaining an access token.
  // 2. Is the access token expired?
  // 3. Is the access token granted the required scopes?
  // 4. Is the user or client giving the grants in the access token authorized to operate the scopes granted?
  // 5. Is the access token revoked?

  // All requests need to be authenticated.
  r.Use(authenticationRequired())

  r.GET("/identities", authorizationRequired("idpbe.identities.get"), identities.GetCollection(env))
  r.POST("/identities", authorizationRequired("idpbe.identities.post"), identities.PostCollection(env))
  r.PUT("/identities", authorizationRequired("idpbe.identities.update"), identities.PutCollection(env))
  r.POST("/identities/authenticate", authorizationRequired("idpbe.authenticate"), identities.PostAuthenticate(env))
  r.POST("/identities/logout", authorizationRequired("idpbe.logout"), identities.PostLogout(env))
  r.POST("/identities/revoke", authorizationRequired("idpbe.revoke"), identities.PostRevoke(env))
  r.POST("/identities/recover", authorizationRequired("idpbe.recover"), identities.PostRecover(env))

  r.RunTLS(":" + config.Self.Port, "/srv/certs/idpbe-cert.pem", "/srv/certs/idpbe-key.pem")
}

func authenticationRequired() gin.HandlerFunc {
  fn := func(c *gin.Context) {
    var requestId string = c.MustGet(requestIdKey).(string)
    debugLog(app, "authenticationRequired", "Checking Authorization: Bearer <token> in request", requestId)

    var token *oauth2.Token
    auth := c.Request.Header.Get("Authorization")
    split := strings.SplitN(auth, " ", 2)
    if len(split) == 2 || strings.EqualFold(split[0], "bearer") {
      debugLog(app, "authenticationRequired", "Authorization: Bearer <token> found for request.", requestId)
      token = &oauth2.Token{
        AccessToken: split[1],
        TokenType: split[0],
      }

      // See #2 of QTNA
      // https://godoc.org/golang.org/x/oauth2#Token.Valid
      if token.Valid() == true {

        // See #5 of QTNA
        // FIXME: Call token revoked list to check if token is revoked.

        debugLog(app, "authenticationRequired", "Valid access token", requestId)
        c.Set(accessTokenKey, token)
        c.Next() // Authentication successful, continue.
        return;
      }

      // Deny by default
      debugLog(app, "authenticationRequired", "Invalid Access token", requestId)
      c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid access token."})
      c.Abort()
      return
    }

    // Deny by default
    debugLog(app, "authenticationRequired", "Missing access token", requestId)
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Authorization: Bearer <token> not found in request."})
    c.Abort()
  }
  return gin.HandlerFunc(fn)
}

func authorizationRequired(requiredScopes ...string) gin.HandlerFunc {
  fn := func(c *gin.Context) {
    var requestId string = c.MustGet(requestIdKey).(string)
    debugLog(app, "authorizationRequired", "Checking Authorization: Bearer <token> in request", requestId)

    accessToken, accessTokenExists := c.Get(accessTokenKey)
    if accessTokenExists == false {
      c.JSON(http.StatusUnauthorized, gin.H{"error": "No access token found. Hint: Is bearer token missing?"})
      c.Abort()
      return
    }
    debugLog(app, "authorizationRequired", "Dumping access_token. DO NOT DO THIS IN PRODUCTION!", requestId)
    fmt.Println(accessToken)

    // FIXME: Implement QTNA #3 and #4

    // See #3 of QTNA
    debugLog(app, "authorizationRequired", "Missing implementation of QTNA #3 - Is the access token granted the required scopes?", requestId)
    // See #4 of QTNA
    debugLog(app, "authorizationRequired", "Missing implementation of QTNA #4 - Is the user or client giving the grants in the access token authorized to operate the scopes granted?", requestId)

    strRequiredScopes := strings.Join(requiredScopes, ",")

    foundRequiredScopes := true
    if foundRequiredScopes {
      debugLog(app, "authorizationRequired", "Valid scopes: " + strRequiredScopes, requestId)
      c.Next() // Authentication successful, continue.
      return;
    }

    // Deny by default
    debugLog(app, "authorizationRequired", "Invalid scopes: " + strRequiredScopes + " Hint: Some required scopes are missing, invalid or not granted", requestId)
    c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid scopes. Hint: Some required scopes are missing, invalid or not granted"})
    c.Abort()
  }
  return gin.HandlerFunc(fn)
}

func debugLog(app string, event string, msg string, requestId string) {
  if requestId == "" {
    fmt.Println(fmt.Sprintf("[app:%s][event:%s] %s", app, event, msg))
    return;
  }
  fmt.Println(fmt.Sprintf("[app:%s][request-id:%s][event:%s] %s", app, requestId, event, msg))
}
