package controller

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	parhamv1 "github.com/parhamds/Email-Operator/api/v1"
)

type EmailSenderConfigReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=parham.my.domain,resources=emailsenderconfigs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=parham.my.domain,resources=emailsenderconfigs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=parham.my.domain,resources=emailsenderconfigs/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=secrets,verbs=get;list;watch

func (r *EmailSenderConfigReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := log.FromContext(ctx)
	log.Info("Reconciling EmailSenderConfig")

	var senderConfig parhamv1.EmailSenderConfig
	if err := r.Get(ctx, req.NamespacedName, &senderConfig); err != nil {
		log.Error(err, "unable to fetch EmailSenderConfig")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if senderConfig.Status.Valid {
		log.Info("SencerConfig already validated, skipping", "email", req.NamespacedName)
		return ctrl.Result{}, nil
	}
	// Send test email to verify the configuration
	_, err := sendEmailMessage(ctx, r.Client, &senderConfig, "parham.dskn@gmail.com", "Test Email", "This is a test email to verify the EmailSenderConfig.")
	if err != nil {
		updateEmailSenderConfigStatus(r, ctx, &senderConfig, false)
		log.Error(err, "failed to send test email", "EmailSenderConfig", senderConfig.Name)
		return ctrl.Result{}, nil
	}
	updateEmailSenderConfigStatus(r, ctx, &senderConfig, true)
	log.Info("Successfully sent test email", "EmailSenderConfig", senderConfig.Name)
	return ctrl.Result{}, nil
}

func updateEmailSenderConfigStatus(r *EmailSenderConfigReconciler, ctx context.Context, config *parhamv1.EmailSenderConfig, status bool) {
	if config.Status.Valid == status {
		return
	}
	config.Status.Valid = status
	if err := r.Status().Update(ctx, config); err != nil {
		log := log.FromContext(ctx)
		log.Error(err, "failed to update EmailSenderConfig status")
	}
}

// SetupWithManager sets up the controller with the Manager.
func (r *EmailSenderConfigReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&parhamv1.EmailSenderConfig{}).
		Complete(r)
}
