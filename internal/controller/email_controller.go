package controller

import (
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parhamv1 "github.com/parhamds/Email-Operator/api/v1"
)

// EmailReconciler reconciles an Email object
type EmailReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=parham.my.domain,resources=emails,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parham.my.domain,resources=emails/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parham.my.domain,resources=emails/finalizers,verbs=update

func (r *EmailReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling Email")

	var email parhamv1.Email
	if err := r.Get(ctx, req.NamespacedName, &email); err != nil {
		log.Error(err, "unable to fetch Email")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	// Check if the email is already sent to avoid resending
	if email.Status.DeliveryStatus == "Sent" {
		log.Info("Email already sent, skipping", "email", req.NamespacedName)
		return ctrl.Result{}, nil
	}

	// Handle deletion if deletion timestamp is set
	if !email.DeletionTimestamp.IsZero() {
		return ctrl.Result{}, nil
	}

	// Fetch the EmailSenderConfig referenced in the Email
	var senderConfig parhamv1.EmailSenderConfig
	if err := r.Get(ctx, client.ObjectKey{Name: email.Spec.SenderConfigRef, Namespace: req.Namespace}, &senderConfig); err != nil {
		log.Info("Unable to fetch EmailSenderConfig", "error", err.Error())
		updateEmailStatus(r, ctx, &email, "Failed", fmt.Sprintf("unable to fetch EmailSenderConfig: %v", err))
		return ctrl.Result{}, nil
	}

	if !senderConfig.Status.Valid {
		err := errors.New("the emailsenderconfig is not valid")
		log.Error(err, "failed to send email")
		updateEmailStatus(r, ctx, &email, "Failed", fmt.Sprintf("failed to send email: %v", err))
		return ctrl.Result{}, nil
	}

	// Send email
	id, err := sendEmailMessage(ctx, r.Client, &senderConfig, email.Spec.RecipientEmail, email.Spec.Subject, email.Spec.Body)
	if err != nil {
		log.Error(err, "failed to send email")
		updateEmailStatus(r, ctx, &email, "Failed", fmt.Sprintf("failed to send email: %v", err))
		return ctrl.Result{}, nil
	}
	email.Status.MessageId = id

	updateEmailStatus(r, ctx, &email, "Sent", "")

	return ctrl.Result{}, nil
}

func updateEmailStatus(r *EmailReconciler, ctx context.Context, email *parhamv1.Email, status, errMsg string) {
	if email.Status.DeliveryStatus == status && email.Status.Error == errMsg {
		return
	}
	email.Status.DeliveryStatus = status
	email.Status.Error = errMsg
	if err := r.Status().Update(ctx, email); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "failed to update Email status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmailReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&parhamv1.Email{}).
		Complete(r)
}
