package auth_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/gorilla/csrf"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	"github.com/concourse/atc/auth"
)

var _ = Describe("AuthCSRFMiddleware", func() {
	simpleHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if auth := r.Header.Get("Authorization"); auth != "" {
			fmt.Fprintf(w, "auth: %s", auth)
		}
	})

	var server *httptest.Server

	BeforeEach(func() {
		protector := csrf.Protect([]byte("shh"))
		server = httptest.NewServer(auth.AuthCSRFMiddleware(protector(simpleHandler)))
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

		Context("with the authorization header", func() {
			BeforeEach(func() {
				request.Header.Set("Authorization", "Bearer: token")
			})

			It("returns 200", func() {
				Expect(response.StatusCode).To(Equal(http.StatusOK))
			})
		})

		Context("without the authorization header", func() {
			It("returns 403", func() {
				Expect(response.StatusCode).To(Equal(http.StatusForbidden))
			})
		})
	})
})
