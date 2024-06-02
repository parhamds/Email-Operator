package controller

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/joho/godotenv"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	parhamv1 "github.com/parhamds/Email-Operator/api/v1"
)

type TestData struct {
	Namespace        string `json:"namespace"`
	MailgunNamespace string `json:"mailgunNamespace"`
	RecipientEmail   string `json:"recipientEmail"`
	EmailSubject     string `json:"emailSubject"`
	EmailBody        string `json:"emailBody"`
	SenderConfigs    []struct {
		Name                string `json:"name"`
		Provider            string `json:"provider"`
		ApiTokenSecretName  string `json:"apiTokenSecretName"`
		SenderEmail         string `json:"senderEmail"`
		ApiTokenSecretValue string `json:"apiTokenSecretValue"`
	} `json:"senderConfigs"`
}

var (
	ctx                    context.Context
	emailNamespacedName    types.NamespacedName
	emailgunNamespacedName types.NamespacedName
	email                  *parhamv1.Email
	emailgun               *parhamv1.Email
	testData               TestData
	senderConfigs          []*parhamv1.EmailSenderConfig
	apiTokenSecrets        []*corev1.Secret
)

func loadTestData() {
	// Load test data from JSON file
	err := godotenv.Load("./../../env")
	Expect(err).NotTo(HaveOccurred())
	data, err := os.ReadFile("testdata.json")
	Expect(err).NotTo(HaveOccurred())

	err = json.Unmarshal(data, &testData)
	Expect(err).NotTo(HaveOccurred())
	tokens, senders := getAPITokens()
	Expect(tokens).NotTo(BeNil())
	Expect(senders).NotTo(BeNil())
	By(tokens[1])
	By(senders[1])

	for i, sc := range testData.SenderConfigs {
		senderConfig := &parhamv1.EmailSenderConfig{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sc.Name,
				Namespace: testData.Namespace,
			},
			Spec: parhamv1.EmailSenderConfigSpec{
				Provider:          sc.Provider,
				ApiTokenSecretRef: sc.ApiTokenSecretName,
				SenderEmail:       senders[i],
			},
		}
		if i == 1 {
			senderConfig.ObjectMeta.Namespace = testData.MailgunNamespace
		}
		senderConfigs = append(senderConfigs, senderConfig)

		apiTokenSecret := &corev1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      sc.ApiTokenSecretName,
				Namespace: testData.Namespace,
			},
			Data: map[string][]byte{
				"apiToken": []byte(tokens[i]),
			},
		}
		if i == 1 {
			apiTokenSecret.ObjectMeta.Namespace = testData.MailgunNamespace
		}
		apiTokenSecrets = append(apiTokenSecrets, apiTokenSecret)
	}
}

var _ = Describe("Email Controller", func() {
	BeforeEach(func() {
		ctx = context.Background()

		emailNamespacedName = types.NamespacedName{
			Name:      "test-email",
			Namespace: testData.Namespace,
		}
		emailgunNamespacedName = types.NamespacedName{
			Name:      "test-email",
			Namespace: testData.MailgunNamespace,
		}

		email = &parhamv1.Email{
			ObjectMeta: metav1.ObjectMeta{
				Name:      emailNamespacedName.Name,
				Namespace: emailNamespacedName.Namespace,
			},
			Spec: parhamv1.EmailSpec{
				SenderConfigRef: senderConfigs[0].Name,
				RecipientEmail:  testData.RecipientEmail,
				Subject:         testData.EmailSubject,
				Body:            testData.EmailBody,
			},
		}

		emailgun = &parhamv1.Email{
			ObjectMeta: metav1.ObjectMeta{
				Name:      emailNamespacedName.Name,
				Namespace: testData.MailgunNamespace,
			},
			Spec: parhamv1.EmailSpec{
				SenderConfigRef: senderConfigs[1].Name,
				RecipientEmail:  testData.RecipientEmail,
				Subject:         testData.EmailSubject,
				Body:            testData.EmailBody,
			},
		}

		By("creating the necessary resources")

		namespaces := []string{testData.Namespace, testData.MailgunNamespace}
		for _, ns := range namespaces {
			namespace := &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: ns,
				},
			}
			err := k8sClient.Create(ctx, namespace)
			if err != nil && !errors.IsAlreadyExists(err) {
				Expect(err).NotTo(HaveOccurred())
			}
		}

		for i := range senderConfigs {
			err := k8sClient.Get(ctx, types.NamespacedName{Name: senderConfigs[i].Name, Namespace: senderConfigs[i].ObjectMeta.Namespace}, senderConfigs[i])
			if err != nil && errors.IsNotFound(err) {
				senderConfigs[i].ResourceVersion = ""
				Expect(k8sClient.Create(ctx, senderConfigs[i])).To(Succeed())
			}
			err = k8sClient.Get(ctx, types.NamespacedName{Name: apiTokenSecrets[i].Name, Namespace: senderConfigs[i].ObjectMeta.Namespace}, apiTokenSecrets[i])
			if err != nil && errors.IsNotFound(err) {
				apiTokenSecrets[i].ResourceVersion = ""
				Expect(k8sClient.Create(ctx, apiTokenSecrets[i])).To(Succeed())
			}

		}

		// Ensure the email resource
		err := k8sClient.Get(ctx, emailNamespacedName, email)
		if err != nil && errors.IsNotFound(err) {
			email.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, email)).To(Succeed())
		}
		err = k8sClient.Get(ctx, emailgunNamespacedName, emailgun)
		if err != nil && errors.IsNotFound(err) {
			emailgun.ResourceVersion = ""
			Expect(k8sClient.Create(ctx, emailgun)).To(Succeed())
		}
	})
	for i := 0; i < 2; i++ {
		It("should validate the Valid EmailSenderConfigs successfully", func() {

			By(string(apiTokenSecrets[i].Data["apiToken"]))
			By("Reconciling the created resource")
			controllerReconciler := &EmailSenderConfigReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: types.NamespacedName{
					Name:      senderConfigs[i].Name,
					Namespace: senderConfigs[i].ObjectMeta.Namespace,
				},
			})
			Expect(err).NotTo(HaveOccurred())

			// Verify the status is updated correctly
			By("Verifying the senderConfig status is updated to 'Valid'")
			Eventually(func() bool {
				err := k8sClient.Get(ctx, types.NamespacedName{
					Name:      senderConfigs[i].Name,
					Namespace: senderConfigs[i].ObjectMeta.Namespace,
				}, senderConfigs[i])
				if err != nil {
					return false
				}
				return senderConfigs[i].Status.Valid
			}, time.Second*10, time.Millisecond*500).Should(BeTrue())

		})
	}

	It("should validate the invalid EmailSenderConfig unsuccessfully", func() {
		By("Reconciling the created resource")
		controllerReconciler := &EmailSenderConfigReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: types.NamespacedName{
				Name:      senderConfigs[2].Name,
				Namespace: senderConfigs[2].ObjectMeta.Namespace,
			},
		})
		Expect(err).NotTo(HaveOccurred())

		// Verify the status is not updated to 'Valid'
		By("Verifying the senderConfig status is not updated to 'Valid'")
		Eventually(func() bool {
			err := k8sClient.Get(ctx, types.NamespacedName{
				Name:      senderConfigs[2].Name,
				Namespace: senderConfigs[2].ObjectMeta.Namespace,
			}, senderConfigs[2])
			if err != nil {
				return false
			}
			return senderConfigs[2].Status.Valid
		}, time.Second*10, time.Millisecond*500).Should(BeFalse())
	})

	AfterEach(func() {
		By("cleaning up the resources")

		// Cleanup email resource
		err := k8sClient.Delete(ctx, email)
		if err != nil && !errors.IsNotFound(err) {
			Fail("Failed to delete Email resource")
		}
	})

	It("should successfully reconcile the resource for the MaileSend senderconfig", func() {

		By("Reconciling the created resource")
		email.Spec.SenderConfigRef = senderConfigs[0].Name
		Expect(k8sClient.Update(ctx, email)).To(Succeed())

		controllerReconciler := &EmailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: emailNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		// Verify the status is updated correctly
		By("Verifying the email status is updated to 'Sent'")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.DeliveryStatus
		}, time.Second*5, time.Millisecond*250).Should(Equal("Sent"))

		By("Verifying the email status error message is set")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.Error
		}, time.Second*5, time.Millisecond*250).Should(Equal(""))

	})

	It("should successfully reconcile the resource for the MailGun senderconfig", func() {

		By("Reconciling the created resource")
		emailgun.Spec.SenderConfigRef = senderConfigs[1].Name
		Expect(k8sClient.Update(ctx, emailgun)).To(Succeed())

		controllerReconciler := &EmailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: emailgunNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		// Verify the status is updated correctly
		By("Verifying the email status is updated to 'Sent'")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailgunNamespacedName, emailgun)
			if err != nil {
				return ""
			}
			return emailgun.Status.DeliveryStatus
		}, time.Second*5, time.Millisecond*250).Should(Equal("Sent"))

		By("Verifying the email status error message is set")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailgunNamespacedName, emailgun)
			if err != nil {
				return ""
			}
			return emailgun.Status.Error
		}, time.Second*5, time.Millisecond*250).Should(Equal(""))

	})

	It("should not successfully reconcile the resource for the Invalid senderconfig", func() {
		By("Reconciling the created resource")
		email.Spec.SenderConfigRef = senderConfigs[2].Name
		Expect(k8sClient.Update(ctx, email)).To(Succeed())

		controllerReconciler := &EmailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: emailNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		// Verify the status is updated correctly
		By("Verifying the email status is updated to 'Failed'")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.DeliveryStatus
		}, time.Second*5, time.Millisecond*250).Should(Equal("Failed"))

		By("Verifying the email status error message is set")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.Error
		}, time.Second*5, time.Millisecond*250).Should(ContainSubstring("the emailsenderconfig is not valid"))
	})

	It("should handle invalid API token correctly", func() {
		By("Updating the API token Secret with invalid data")
		apiTokenSecrets[0].Data["apiToken"] = []byte("invalid-api-token")
		Expect(k8sClient.Update(ctx, apiTokenSecrets[0])).To(Succeed())

		By("Reconciling the resource again")
		controllerReconciler := &EmailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: emailNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		// Verify the status is updated correctly
		By("Verifying the email status is updated to 'Failed'")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.DeliveryStatus
		}, time.Second*5, time.Millisecond*250).Should(Equal("Failed"))

		// Verify the error message is set
		By("Verifying the email status error message is set")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.Error
		}, time.Second*5, time.Millisecond*250).Should(ContainSubstring("401 Unauthenticated"))
	})

	It("should handle missing Secret correctly", func() {
		By("Deleting the API token Secret")
		Expect(k8sClient.Delete(ctx, apiTokenSecrets[0])).To(Succeed())

		By("Reconciling the resource again")
		controllerReconciler := &EmailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: emailNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		// Verify the status is updated correctly
		By("Verifying the email status is updated to 'Failed'")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.DeliveryStatus
		}, time.Second*5, time.Millisecond*250).Should(Equal("Failed"))

		By("Verifying the email status error message is set")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.Error
		}, time.Second*5, time.Millisecond*250).Should(ContainSubstring("unable to fetch secret"))
	})

	It("should handle invalid receiver email format correctly", func() {
		By("Ensuring the email resource is not marked for deletion")
		err := k8sClient.Get(ctx, emailNamespacedName, email)
		Expect(err).NotTo(HaveOccurred())
		By(fmt.Sprintf("Email before update: %+v", email))

		// Ensure the email resource is not marked for deletion
		if !email.DeletionTimestamp.IsZero() {
			email.DeletionTimestamp = nil
			email.DeletionGracePeriodSeconds = nil
			Expect(k8sClient.Update(ctx, email)).To(Succeed())
		}

		By("Updating the Email resource with an invalid receiver email format")
		email.Spec.RecipientEmail = "invalid-email-format"
		Expect(k8sClient.Update(ctx, email)).To(Succeed())

		By("Reconciling the resource again")
		controllerReconciler := &EmailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err = controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: emailNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		By("Verifying the email status is updated to 'Failed'")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				By(fmt.Sprint(err))
				return ""
			}
			By(fmt.Sprintf("Email after reconciliation: %+v", email))
			return email.Status.DeliveryStatus
		}, time.Second*5, time.Millisecond*250).Should(Equal("Failed"))

		By("Verifying the email status error message is set")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.Error
		}, time.Second*5, time.Millisecond*250).Should(ContainSubstring("email must be a valid email address"))
	})

	It("should handle missing emailsenderconfig correctly", func() {
		By("Deleting the emailsenderconfig")
		Expect(k8sClient.Delete(ctx, senderConfigs[0])).To(Succeed())

		By("Reconciling the resource again")
		controllerReconciler := &EmailReconciler{
			Client: k8sClient,
			Scheme: k8sClient.Scheme(),
		}

		_, err := controllerReconciler.Reconcile(ctx, reconcile.Request{
			NamespacedName: emailNamespacedName,
		})
		Expect(err).NotTo(HaveOccurred())

		// Verify the status is updated correctly
		By("Verifying the email status is updated to 'Failed'")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.DeliveryStatus
		}, time.Second*5, time.Millisecond*250).Should(Equal("Failed"))

		By("Verifying the email status error message is set")
		Eventually(func() string {
			err := k8sClient.Get(ctx, emailNamespacedName, email)
			if err != nil {
				return ""
			}
			return email.Status.Error
		}, time.Second*5, time.Millisecond*250).Should(ContainSubstring("unable to fetch EmailSenderConfig"))
	})

})
