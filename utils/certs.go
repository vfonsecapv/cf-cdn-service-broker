package utils

import (
	"path"
	"strings"

	"crypto"
	"crypto/rand"
	"crypto/rsa"

	"github.com/xenolf/lego/acme"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"

	"github.com/18F/cf-cdn-service-broker/config"
)

type User struct {
	Email        string
	Registration *acme.RegistrationResource
	key          crypto.PrivateKey
}

func (u *User) GetEmail() string {
	return u.Email
}

func (u *User) GetRegistration() *acme.RegistrationResource {
	return u.Registration
}

func (u *User) GetPrivateKey() crypto.PrivateKey {
	return u.key
}

type HTTPProvider struct {
	settings config.Settings
}

func (p *HTTPProvider) Present(domain, token, keyAuth string) error {
	svc := s3.New(session.New(&aws.Config{Region: aws.String(p.settings.AwsRegion)}))

	_, err := svc.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(p.settings.Bucket),
		Body:   strings.NewReader(keyAuth),
		Key:    aws.String(path.Join(".well-known", "acme-challenge", token)),
	})

	return err
}

func (p *HTTPProvider) CleanUp(domain, token, keyAuth string) error {
	svc := s3.New(session.New(&aws.Config{Region: aws.String(p.settings.AwsRegion)}))

	_, err := svc.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(p.settings.Bucket),
		Key:    aws.String(path.Join(".well-known", "acme-challenge", token)),
	})

	return err
}

func ObtainCertificate(settings config.Settings, domain string) (acme.CertificateResource, error) {
	keySize := 2048
	key, err := rsa.GenerateKey(rand.Reader, keySize)
	if err != nil {
		return acme.CertificateResource{}, err
	}

	user := User{
		Email: settings.Email,
		key:   key,
	}
	client, err := acme.NewClient(settings.AcmeUrl, &user, acme.RSA2048)

	client.SetChallengeProvider(acme.HTTP01, &HTTPProvider{settings: settings})
	client.ExcludeChallenges([]acme.Challenge{acme.DNS01, acme.TLSSNI01})

	reg, err := client.Register()
	user.Registration = reg

	err = client.AgreeToTOS()

	domains := []string{domain}
	certificate, failures := client.ObtainCertificate(domains, true, nil)

	if len(failures) > 0 {
		return acme.CertificateResource{}, failures[domain]
	}

	return certificate, nil
}