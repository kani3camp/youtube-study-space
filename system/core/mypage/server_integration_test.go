//go:build integration

package mypage

import (
 "context"
 "crypto/rand"
 "crypto/rsa"
 "encoding/json"
 "net/http"
 "net/http/httptest"
 "net/url"
 "strings"
 "testing"
 "time"

 "app.modules/internal/integrationtest"
 "cloud.google.com/go/firestore"
 "firebase.google.com/go/v4/auth"
 "github.com/stretchr/testify/require"
 "google.golang.org/api/option"
 "google.golang.org/grpc/codes"
 "google.golang.org/grpc/status"
)

func TestConstructedServerOAuthConfirmAndSessionCompleteWithSignedAppCheckAndEmulator(t *testing.T){
 integrationtest.RequireFirestoreEmulator(t)
 ctx:=context.Background();project:="demo-youtube-study-space-ci"
 client,err:=firestore.NewClient(ctx,project,option.WithoutAuthentication());require.NoError(t,err);t.Cleanup(func(){require.NoError(t,client.Close())})
 now:=time.Now().UTC().Truncate(time.Second);uid:="UCserver"+strings.Repeat("0",15)+"1";key,err:=rsa.GenerateKey(rand.Reader,2048);require.NoError(t,err)
 claims:=syntheticAppClaims(now);appToken:=syntheticAppCheck(t,key,"synthetic-key",claims,"JWT");jwks:=publicJWKS(t,key,"synthetic-key")
 sdk:=&fakeFirebaseClient{token:&auth.Token{UID:uid,Subject:uid,Issuer:"https://securetoken.google.com/"+project,Audience:project,Expires:now.Add(time.Hour).Unix(),IssuedAt:now.Unix(),AuthTime:now.Unix(),Firebase:auth.FirebaseInfo{SignInProvider:"custom"}}}
 oauth:=&fakeOAuth{channels:[]Channel{{ID:uid,DisplayName:"Synthetic server channel"}}}
 handler,err:=NewMyPageServer(ServerConfig{Environment:"development",ProjectID:project,ProjectNumber:"123456789",WebAppID:"1:123456789:web:synthetic",PublicOrigin:"https://example.invalid",Policy:Policy{Privacy:"synthetic-p",Terms:"synthetic-t"}},ServerDependencies{Firestore:client,Firebase:sdk,OAuth:oauth,Now:func()time.Time{return now},AppCheckHTTP:&http.Client{Transport:providerTransport(func(*http.Request)(*http.Response,error){return providerResponse(200,jwks),nil})}});require.NoError(t,err)
 call:=func(method,path,body string,cookie *http.Cookie,authenticated bool)*httptest.ResponseRecorder{
  r:=httptest.NewRequest(method,"https://example.invalid"+path,strings.NewReader(body));r.RemoteAddr="127.0.0.1:1234";r.Header.Set("Origin","https://example.invalid");if body!=""{r.Header.Set("Content-Type","application/json")};r.Header.Set("X-Firebase-AppCheck",appToken);if cookie!=nil{r.AddCookie(cookie)};if authenticated{r.Header.Set("Authorization","Bearer synthetic-id-token")};w:=httptest.NewRecorder();handler.ServeHTTP(w,r);return w
 }
 started:=call("POST","/api/auth/youtube/start",`{"privacyPolicyVersion":"synthetic-p","privacyAccepted":true,"termsVersion":"synthetic-t","termsAccepted":true}`,nil,false);require.Equal(t,200,started.Code)
 var start StartResponse;require.NoError(t,json.Unmarshal(started.Body.Bytes(),&start));target,err:=url.Parse(start.AuthorizationURL);require.NoError(t,err);cookies:=started.Result().Cookies();require.Len(t,cookies,1);cookie:=cookies[0]
 t.Cleanup(func(){_,err:=client.Collection("oauth-transactions").Doc(cookie.Value).Delete(ctx);require.NoError(t,err);_,err=client.Collection("web-accounts").Doc(uid).Delete(ctx);require.NoError(t,err)})
 callback:=call("GET","/api/auth/youtube/callback?code=synthetic-code&state="+target.Query().Get("state"),"",cookie,false);require.Equal(t,302,callback.Code);require.Equal(t,int32(1),oauth.calls.Load())
 channel:=call("GET","/api/auth/youtube/channel","",cookie,false);require.Equal(t,200,channel.Code);var confirmation ChannelResponse;require.NoError(t,json.Unmarshal(channel.Body.Bytes(),&confirmation))
 confirmed:=call("POST","/api/auth/youtube/confirm",`{"confirmationRef":"`+confirmation.ConfirmationRef+`"}`,cookie,false);require.Equal(t,200,confirmed.Code);require.Equal(t,1,sdk.mintCalls)
 repeat:=call("POST","/api/auth/youtube/confirm",`{"confirmationRef":"`+confirmation.ConfirmationRef+`"}`,cookie,false);require.NotEqual(t,200,repeat.Code);require.Equal(t,1,sdk.mintCalls)
 completed:=call("POST","/api/auth/session/complete","",nil,true);require.Equal(t,204,completed.Code)
 page:=call("GET","/api/mypage","",nil,true);require.Equal(t,200,page.Code);require.Contains(t,page.Body.String(),"Synthetic server channel")
 _,err=client.Collection("users").Doc(uid).Get(ctx);require.Equal(t,codes.NotFound,status.Code(err))
 sdk.token.Firebase.SignInProvider="google.com";require.Equal(t,401,call("GET","/api/mypage","",nil,true).Code)
}
