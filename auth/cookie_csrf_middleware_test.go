package auth_test

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/csrf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
)

var _ = Describe("CookieCSRFMiddleware", func() {
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			fmt.Fprintf(w, "auth: %s", auth)
		}
	})

	var server *httptest.Server

	BeforeEach(func() {
		protector := csrf.Protect([]byte("shh"))
		server = httptest.NewServer(auth.CookieCSRFMiddleware(protector(simpleHandler)))
	})

	AfterEach(func() {
		server.Close()
	})

	Describe("handling a request", func() {
		var request *http.Request
		var response *http.Response

		BeforeEach(func() {
			var err error
			request, err = http.NewRequest("POST", server.URL, nil)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			var err error
			response, err = http.DefaultClient.Do(request)
			Expect(err).NotTo(HaveOccurred())
		})

		It("proxies to the handler without setting the Authorization header", func() {
			responseBody, err := ioutil.ReadAll(response.Body)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(responseBody)).To(Equal(""))
		})

		Context("without the ATC-Authorization cookie", func() {
			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("with the ATC-Authorization cookie", func() {
			BeforeEach(func() {
				request.AddCookie(&http.Cookie{
					Name:  auth.CookieName,
					Value: header("username", "password"),
				})
			})

			It("returns 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})
	})
})
