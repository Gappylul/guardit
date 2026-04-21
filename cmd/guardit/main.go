package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"

	admissionv1 "k8s.io/api/admission/v1"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/gappylul/guardit/pkg/sdk"
)

func main() {
	p := loadPolicy()
	log.Printf("guardit: policy=%q replicaLimit=%d registries=%v",
		p.Metadata.Name, p.Spec.ReplicaLimit, p.Spec.AllowedRegistries)

	mux := http.NewServeMux()
	mux.HandleFunc("/validate", makeHandler(p))
	mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	tlsCert := os.Getenv("TLS_CERT_FILE")
	tlsKey := os.Getenv("TLS_KEY_FILE")

	if tlsCert == "" || tlsKey == "" {
		log.Println("⚠  TLS_CERT_FILE / TLS_KEY_FILE not set - running plain HTTP on :8080")
		log.Fatal(http.ListenAndServe(":8080", mux))
	} else {
		log.Println("listening on :8443 (TLS)")
		log.Fatal(http.ListenAndServeTLS(":8443", tlsCert, tlsKey, mux))
	}
}

func loadPolicy() *sdk.Policy {
	if path := os.Getenv("GUARDIT_POLICY"); path != "" {
		p, err := sdk.LoadFromFile(path)
		if err != nil {
			log.Fatalf("load policy %s: %v", path, err)
		}
		return p
	}
	if p, err := sdk.Discover("/etc/guardit"); err == nil {
		return p
	}
	log.Println("no guardit.yaml found - using built-in defaults")
	return sdk.Default()
}

func makeHandler(p *sdk.Policy) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, "read body", http.StatusBadRequest)
			return
		}

		var review admissionv1.AdmissionReview
		if err := json.Unmarshal(body, &review); err != nil {
			http.Error(w, "parse AdmissionReview", http.StatusBadRequest)
			return
		}

		review.Response = admit(review.Request, p)
		review.Response.UID = review.Request.UID

		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(review); err != nil {
			log.Printf("encode response: %v", err)
		}
	}
}

func admit(req *admissionv1.AdmissionRequest, p *sdk.Policy) *admissionv1.AdmissionResponse {
	var deployment appsv1.Deployment
	if err := json.Unmarshal(req.Object.Raw, &deployment); err != nil {
		return denyWith("ParseError", fmt.Sprintf("could not parse Deployment: %v", err))
	}

	dreq := sdk.FromDeployment(&deployment)
	result := sdk.Evaluate(p, dreq)

	if result.Allowed {
		return &admissionv1.AdmissionResponse{Allowed: true}
	}

	msgs := make([]string, len(result.Violations))
	for i, v := range result.Violations {
		msgs[i] = fmt.Sprintf("[%s] %s", v.Code, v.Message)
	}
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Code:    http.StatusForbidden,
			Message: strings.Join(msgs, "; "),
		},
	}
}

func denyWith(code, message string) *admissionv1.AdmissionResponse {
	return &admissionv1.AdmissionResponse{
		Allowed: false,
		Result: &metav1.Status{
			Code:    http.StatusForbidden,
			Message: fmt.Sprintf("[%s] %s", code, message),
		},
	}
}
