package controller

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/mailersend/mailersend-go"
	mailgun "github.com/mailgun/mailgun-go/v4"
	parhamv1 "github.com/parhamds/Email-Operator/api/v1"
)

func isValidEmail(email string) bool {
	re := regexp.MustCompile(`^[a-z0-9._%+\-]+@[a-z0-9.\-]+\.[a-z]{2,}$`)
	return re.MatchString(email)
}

func sendEmailMessage(ctx context.Context, c client.Client, senderConfig *parhamv1.EmailSenderConfig, recipientEmail, subject, body string) (string, error) {
	var id string
	if !isValidEmail(recipientEmail) {
		return "", fmt.Errorf("email must be a valid email address")
	}

	apiToken, err := getApiTokenFromSecret(ctx, c, senderConfig.Namespace, senderConfig.Spec.ApiTokenSecretRef, "apiToken")
	if err != nil {
		return "", fmt.Errorf("failed to retrieve API token: %v", err)
	}

	if senderConfig.Spec.Provider == "MailerSend" {
		id, err = sendEmailMessageMailerSend(ctx, senderConfig, recipientEmail, subject, body, apiToken)
		if err != nil {
			return "", fmt.Errorf("failed to send email: %v", err)
		}
	} else {
		id, err = sendEmailMessageMailgun(ctx, senderConfig, recipientEmail, subject, body, apiToken)
		if err != nil {
			return "", fmt.Errorf("failed to send email: %v", err)
		}
	}
	return id, nil
}

func sendEmailMessageMailerSend(ctx context.Context, senderConfig *parhamv1.EmailSenderConfig, recipientEmail, subject, body, apiToken string) (string, error) {

	ms := mailersend.NewMailersend(apiToken)

	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	from := mailersend.From{
		Name:  senderConfig.Spec.SenderEmail,
		Email: senderConfig.Spec.SenderEmail,
	}

	recipients := []mailersend.Recipient{
		{
			Name:  "Recipient",
			Email: recipientEmail,
		},
	}

	message := ms.Email.NewMessage()
	message.SetFrom(from)
	message.SetRecipients(recipients)
	message.SetSubject(subject)
	message.SetText(body)

	res, err := ms.Email.Send(sendCtx, message)
	if err != nil {
		return "", err
	}

	defer res.Body.Close()

	if res.StatusCode != http.StatusAccepted {
		return "", fmt.Errorf("error response from MailerSend: %s", res.Status)
	}

	id := res.Header.Get("X-Message-Id")

	return id, nil
}

func sendEmailMessageMailgun(ctx context.Context, senderConfig *parhamv1.EmailSenderConfig, recipientEmail, subject, body, apiToken string) (string, error) {

	parts := strings.Split(senderConfig.Spec.SenderEmail, "@")
	if len(parts) != 2 {
		return "", errors.New("invalid sender email format")
	}

	mg := mailgun.NewMailgun(parts[1], apiToken)
	sendCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	from := senderConfig.Spec.SenderEmail

	m := mg.NewMessage(
		from,
		subject,
		body,
		recipientEmail,
	)

	_, id, err := mg.Send(sendCtx, m)

	if err != nil {
		return "", err
	}

	return id, nil
}

func getApiTokenFromSecret(ctx context.Context, c client.Client, namespace, secretName, secretKey string) (string, error) {
	var secret corev1.Secret
	if err := c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: namespace}, &secret); err != nil {
		if err := c.Get(ctx, client.ObjectKey{Name: secretName, Namespace: "default"}, &secret); err != nil {
			return "", fmt.Errorf("unable to fetch secret %s: %v", secretName, err)
		}
	}

	tokenBase64, ok := secret.Data[secretKey]
	if !ok {
		return "", fmt.Errorf("secret %s does not contain key %s", secretName, secretKey)
	}

	return string(tokenBase64), nil
}
