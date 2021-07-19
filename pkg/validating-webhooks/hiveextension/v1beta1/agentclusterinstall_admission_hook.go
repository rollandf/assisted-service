package v1beta1

import (
	"fmt"
	"net/http"

	hiveext "github.com/openshift/assisted-service/api/hiveextension/v1beta1"
	log "github.com/sirupsen/logrus"
	admissionv1beta1 "k8s.io/api/admission/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	agentClusterInstallGroup    = "extensions.hive.openshift.io"
	agentClusterInstallVersion  = "v1beta1"
	agentClusterInstallResource = "agentclusterinstalls"

	agentClusterInstallAdmissionGroup   = "admission.agentinstall.openshift.io"
	agentClusterInstallAdmissionVersion = "v1"
)

// AgentClusterInstallValidatingAdmissionHook is a struct that is used to reference what code should be run by the generic-admission-server.
type AgentClusterInstallValidatingAdmissionHook struct {
	decoder *admission.Decoder
}

// NewAgentClusterInstallValidatingAdmissionHook constructs a new AgentClusterInstallValidatingAdmissionHook
func NewAgentClusterInstallValidatingAdmissionHook(decoder *admission.Decoder) *AgentClusterInstallValidatingAdmissionHook {
	return &AgentClusterInstallValidatingAdmissionHook{decoder: decoder}
}

// ValidatingResource is called by generic-admission-server on startup to register the returned REST resource through which the
//                    webhook is accessed by the kube apiserver.
// For example, generic-admission-server uses the data below to register the webhook on the REST resource "/apis/admission.agentinstall.openshift.io/v1/agentclusterinstalls".
//              When the kube apiserver calls this registered REST resource, the generic-admission-server calls the Validate() method below.
func (a *AgentClusterInstallValidatingAdmissionHook) ValidatingResource() (plural schema.GroupVersionResource, singular string) {
	log.WithFields(log.Fields{
		"group":    agentClusterInstallAdmissionGroup,
		"version":  agentClusterInstallAdmissionVersion,
		"resource": "agentclusterinstallvalidator",
	}).Info("Registering validation REST resource")
	// NOTE: This GVR is meant to be different than the AgentClusterInstall CRD GVR which has group "hivextension.openshift.io".
	return schema.GroupVersionResource{
			Group:    agentClusterInstallAdmissionGroup,
			Version:  agentClusterInstallAdmissionVersion,
			Resource: "agentclusterinstallvalidators",
		},
		"agentclusterinstallvalidator"
}

// Initialize is called by generic-admission-server on startup to setup any special initialization that your webhook needs.
func (a *AgentClusterInstallValidatingAdmissionHook) Initialize(kubeClientConfig *rest.Config, stopCh <-chan struct{}) error {
	log.WithFields(log.Fields{
		"group":    agentClusterInstallAdmissionGroup,
		"version":  agentClusterInstallAdmissionVersion,
		"resource": "agentclusterinstallvalidator",
	}).Info("Initializing validation REST resource")
	return nil // No initialization needed right now.
}

// Validate is called by generic-admission-server when the registered REST resource above is called with an admission request.
// Usually it's the kube apiserver that is making the admission validation request.
func (a *AgentClusterInstallValidatingAdmissionHook) Validate(admissionSpec *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	contextLogger := log.WithFields(log.Fields{
		"operation": admissionSpec.Operation,
		"group":     admissionSpec.Resource.Group,
		"version":   admissionSpec.Resource.Version,
		"resource":  admissionSpec.Resource.Resource,
		"method":    "Validate",
	})

	if !a.shouldValidate(admissionSpec) {
		contextLogger.Info("Skipping validation for request")
		// The request object isn't something that this validator should validate.
		// Therefore, we say that it's allowed.
		return &admissionv1beta1.AdmissionResponse{
			Allowed: true,
		}
	}

	contextLogger.Info("Validating request")

	if admissionSpec.Operation == admissionv1beta1.Create {
		return a.validateCreate(admissionSpec)
	}

	if admissionSpec.Operation == admissionv1beta1.Update {
		return a.validateUpdate(admissionSpec)
	}

	// We're only validating creates and updates at this time, so all other operations are explicitly allowed.
	contextLogger.Info("Successful validation")
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}
}

// shouldValidate explicitly checks if the request should validated. For example, this webhook may have accidentally been registered to check
// the validity of some other type of object with a different GVR.
func (a *AgentClusterInstallValidatingAdmissionHook) shouldValidate(admissionSpec *admissionv1beta1.AdmissionRequest) bool {
	contextLogger := log.WithFields(log.Fields{
		"operation": admissionSpec.Operation,
		"group":     admissionSpec.Resource.Group,
		"version":   admissionSpec.Resource.Version,
		"resource":  admissionSpec.Resource.Resource,
		"method":    "shouldValidate",
	})

	if admissionSpec.Resource.Group != agentClusterInstallGroup {
		contextLogger.Debug("Returning False, not our group")
		return false
	}

	if admissionSpec.Resource.Version != agentClusterInstallVersion {
		contextLogger.Debug("Returning False, it's our group, but not the right version")
		return false
	}

	if admissionSpec.Resource.Resource != agentClusterInstallResource {
		contextLogger.Debug("Returning False, it's our group and version, but not the right resource")
		return false
	}

	// If we get here, then we're supposed to validate the object.
	contextLogger.Debug("Returning True, passed all prerequisites.")
	return true
}

// validateCreate specifically validates create operations for AgentClusterInstall objects.
func (a *AgentClusterInstallValidatingAdmissionHook) validateCreate(admissionSpec *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	contextLogger := log.WithFields(log.Fields{
		"operation": admissionSpec.Operation,
		"group":     admissionSpec.Resource.Group,
		"version":   admissionSpec.Resource.Version,
		"resource":  admissionSpec.Resource.Resource,
		"method":    "validateCreate",
	})

	aci := &hiveext.AgentClusterInstall{}
	if err := a.decoder.DecodeRaw(admissionSpec.Object, aci); err != nil {
		contextLogger.Errorf("Failed unmarshaling Object: %v", err.Error())
		return &admissionv1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status: metav1.StatusFailure, Code: http.StatusBadRequest, Reason: metav1.StatusReasonBadRequest,
				Message: err.Error(),
			},
		}
	}

	// Add the new data to the contextLogger
	contextLogger.Data["object.Name"] = aci.Name

	allErrs := field.ErrorList{}

	//TODO temporary check for test only
	if len(aci.Name) > validation.DNS1123LabelMaxLength {
		message := fmt.Sprintf("Invalid cluster deployment name (.meta.name): %s", validation.MaxLenError(validation.DNS1123LabelMaxLength))
		contextLogger.Error(message)
		return &admissionv1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status: metav1.StatusFailure, Code: http.StatusBadRequest, Reason: metav1.StatusReasonBadRequest,
				Message: message,
			},
		}
	}

	if len(allErrs) > 0 {
		statusError := errors.NewInvalid(aci.GroupVersionKind().GroupKind(), aci.Name, allErrs).Status()
		contextLogger.Infof(statusError.Message)
		return &admissionv1beta1.AdmissionResponse{
			Allowed: false,
			Result:  &statusError,
		}
	}

	// If we get here, then all checks passed, so the object is valid.
	contextLogger.Info("Successful validation")
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}
}

// validateUpdate specifically validates update operations for AgentClusterInstall objects.
func (a *AgentClusterInstallValidatingAdmissionHook) validateUpdate(admissionSpec *admissionv1beta1.AdmissionRequest) *admissionv1beta1.AdmissionResponse {
	contextLogger := log.WithFields(log.Fields{
		"operation": admissionSpec.Operation,
		"group":     admissionSpec.Resource.Group,
		"version":   admissionSpec.Resource.Version,
		"resource":  admissionSpec.Resource.Resource,
		"method":    "validateUpdate",
	})

	newObject := &hiveext.AgentClusterInstall{}
	if err := a.decoder.DecodeRaw(admissionSpec.Object, newObject); err != nil {
		contextLogger.Errorf("Failed unmarshaling Object: %v", err.Error())
		return &admissionv1beta1.AdmissionResponse{
			Allowed: false,
			Result: &metav1.Status{
				Status: metav1.StatusFailure, Code: http.StatusBadRequest, Reason: metav1.StatusReasonBadRequest,
				Message: err.Error(),
			},
		}
	}

	// Add the new data to the contextLogger
	contextLogger.Data["object.Name"] = newObject.Name

	allErrs := field.ErrorList{}
	//TODO Add validation

	if len(allErrs) > 0 {
		statusError := errors.NewInvalid(newObject.GroupVersionKind().GroupKind(), newObject.Name, allErrs).Status()
		contextLogger.Infof(statusError.Message)
		return &admissionv1beta1.AdmissionResponse{
			Allowed: false,
			Result:  &statusError,
		}
	}

	// If we get here, then all checks passed, so the object is valid.
	contextLogger.Info("Successful validation")
	return &admissionv1beta1.AdmissionResponse{
		Allowed: true,
	}
}
