package main

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strings"
)

func init() {
	resourceName := os.Getenv("ResourceName")
	resourceSecret := os.Getenv("ResourceSecret")

	basicString := fmt.Sprintf("%s:%s", resourceName, resourceSecret)
	basicEncoded := base64.StdEncoding.EncodeToString([]byte(basicString))
	basicHeader = fmt.Sprintf("Basic %s", basicEncoded)
}

var basicHeader string

type IntrospectionResult struct {
	Active bool   `json:"active"`
	Sub    string `json:"sub"`
}

type CustomClaims struct {
	Scope []string `json:"scope"`
}

const introspectionEndpoint string = "https://auth.voidwell.com/connect/introspect"
const publishScope string = "voidwell-messagewell-publish"

func tokenAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := getBearerToken(r)
		if token == "" /*|| !validateIntrospection(token)*/ {
			log.Printf("Missing or invalid token")
			http.Error(w, http.StatusText(401), 401)
			return
		}
		if !validateScope(token, publishScope) {
			log.Printf("Invalid scope")
			http.Error(w, http.StatusText(403), 403)
			return
		}
		next.ServeHTTP(w, r)
	})
}

func getBearerToken(r *http.Request) string {
	reqToken := r.Header.Get("Authorization")
	if reqToken != "" {
		splitToken := strings.Split(reqToken, "Bearer ")
		if len(splitToken) == 2 {
			return splitToken[1]
		}
	}
	return ""
}

func validateIntrospection(token string) bool {
	req, _ := http.NewRequest("POST", introspectionEndpoint, nil)
	req.Header.Add("content-type", "application/x-www-form-urlencoded")
	req.Header.Add("Authorization", basicHeader)
	req.Form.Add("token", token)

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Printf("introspection error: %v", err)
		return false
	}

	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)

	var result IntrospectionResult
	json.Unmarshal(body, &result)

	return result.Active
}

func validateScope(token string, scope string) bool {
	claims := extractClaims(token)
	if claims == nil {
		return false
	}
	scopes := claims.Scope
	for _, s := range scopes {
		if s == scope {
			return true
		}
	}
	return false
}

func extractClaims(tokenStr string) *CustomClaims {
	tokenParts := strings.Split(tokenStr, ".")
	payload, err := base64.RawURLEncoding.DecodeString(tokenParts[1])
	if err != nil {
		log.Printf("%v", err)
		return nil
	}

	var claims *CustomClaims
	json.Unmarshal(payload, &claims)

	return claims
}
