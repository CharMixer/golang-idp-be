package identities

import (
  "net/http"
  "github.com/sirupsen/logrus"
  "github.com/gin-gonic/gin"
  "golang-idp-be/config"
  "golang-idp-be/environment"
  "golang-idp-be/gateway/idpapi"
)

type TwoFactorRequest struct {
  Id       string `json:"id" binding:"required"`
  Required2Fa bool   `json:"require_2fa,omitempty" binding:"required"`
  Secret2Fa   string `json:"secret_2fa,omitempty" binding:"required"`
}

type TwoFactorResponse struct {
  Id string `json:"id" binding:"required"`
}

func Post2Fa(env *environment.State, route environment.Route) gin.HandlerFunc {
  fn := func(c *gin.Context) {

    log := c.MustGet(environment.LogKey).(*logrus.Entry)
    log = log.WithFields(logrus.Fields{
      "func": "Post2Fa",
    })

    var input TwoFactorRequest
    err := c.BindJSON(&input)
    if err != nil {
      c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
      c.Abort()
      return
    }

    identities, err := idpapi.FetchIdentitiesForSub(env.Driver, input.Id)
    if err != nil {
      log.Debug(err.Error())
      c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
      c.Abort()
      return;
    }

    if identities != nil {

      identity := identities[0]; // FIXME do not return a list of identities!

      encryptedSecret, err := idpapi.Encrypt(input.Secret2Fa, config.GetString("2fa.cryptkey"))
      if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        c.Abort()
        return
      }

      updatedIdentity, err := idpapi.UpdateTwoFactor(env.Driver, idpapi.Identity{
        Id: identity.Id,
        Require2Fa: input.Required2Fa,
        Secret2Fa: encryptedSecret,
      })
      if err != nil {
        log.Debug(err.Error())
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        c.Abort()
        return;
      }

      c.JSON(http.StatusOK, gin.H{"id": updatedIdentity.Id})
      return
    }

    // Deny by default
    log.WithFields(logrus.Fields{"id": input.Id}).Info("Identity not found")
    c.JSON(http.StatusNotFound, gin.H{"error": "Identity not found"})
  }
  return gin.HandlerFunc(fn)
}