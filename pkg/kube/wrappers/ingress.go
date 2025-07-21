package wrappers

import (
	"net/url"
	"context"
	
	"github.com/stakater/Forecastle/v1/pkg/annotations"
	"github.com/stakater/Forecastle/v1/pkg/log"
	v1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
    corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
)

var (
	logger = log.New()
)

// IngressWrapper struct wraps a kubernetes ingress object
type IngressWrapper struct {
	ingress *v1.Ingress
}

// NewIngressWrapper func creates an instance of IngressWrapper
func NewIngressWrapper(ingress *v1.Ingress) *IngressWrapper {
	return &IngressWrapper{
		ingress: ingress,
	}
}

// GetAnnotationValue extracts an annotation's value present on the ingress wrapped by the object
func (iw *IngressWrapper) GetAnnotationValue(annotationKey string) string {
	if value, ok := iw.ingress.Annotations[annotationKey]; ok {
		return value
	}
	return ""
}

// GetName func extracts name of the ingress wrapped by the object
func (iw *IngressWrapper) GetName() string {
	if nameFromAnnotation := iw.GetAnnotationValue(annotations.ForecastleAppNameAnnotation); nameFromAnnotation != "" {
		return nameFromAnnotation
	}
	return iw.ingress.ObjectMeta.Name
}

// GetNamespace func extracts namespace of the ingress wrapped by the object
func (iw *IngressWrapper) GetNamespace() string {
	return iw.ingress.ObjectMeta.Namespace
}

// GetGroup func extracts group name from the ingress
func (iw *IngressWrapper) GetGroup() string {
	if groupFromAnnotation := iw.GetAnnotationValue(annotations.ForecastleGroupAnnotation); groupFromAnnotation != "" {
		return groupFromAnnotation
	}
	return iw.GetNamespace()
}

func (iw *IngressWrapper) GetProperties() map[string]string {
	if propertiesFromAnnotations := iw.GetAnnotationValue(annotations.ForecastlePropertiesAnnotation); propertiesFromAnnotations != "" {
		return makeMap(propertiesFromAnnotations)
	}
	return nil
}

func GetNamespacesByLabel(client kubernetes.Interface, labelSelector string) ([]corev1.Namespace, error) {
    nsList, err := client.CoreV1().Namespaces().List(context.TODO(), metav1.ListOptions{
        LabelSelector: labelSelector,
    })
    if err != nil {
        return nil, err
    }
    return nsList.Items, nil
}

func GetIngressesFromNamespaces(client kubernetes.Interface, namespaces []corev1.Namespace) ([]networkingv1.Ingress, error) {
    var ingresses []networkingv1.Ingress
    for _, ns := range namespaces {
        ingList, err := client.NetworkingV1().Ingresses(ns.Name).List(context.TODO(), metav1.ListOptions{})
        if err != nil {
            return nil, err
        }
        ingresses = append(ingresses, ingList.Items...)
    }
    return ingresses, nil
}

// GetURL func extracts url of the ingress wrapped by the object
func (iw *IngressWrapper) GetURL() string {

	if urlFromAnnotation := iw.GetAnnotationValue(annotations.ForecastleURLAnnotation); urlFromAnnotation != "" {
		parsedURL, err := url.ParseRequestURI(urlFromAnnotation)
		if err != nil {
			logger.Warn(err)
			return ""
		}
		return parsedURL.String()
	}

	var url string

	if host, exists := iw.tryGetTLSHost(); exists { // Get TLS Host if defined
		url = "https://" + host
	} else if host, exists := iw.tryGetRuleHost(); exists { // Fallback to normal host if defined
		url = "http://" + host
	} else if host, exists := iw.tryGetStatusHost(); exists { // Fallback to status host if defined
		url = "http://" + host
	} else {
		logger.Warn("Unable to infer host for ingress: ", iw.ingress.GetName())
		return ""
	}

	// Append path if defined
	url += iw.getIngressSubPath()

	return url
}

func (iw *IngressWrapper) supportsTLS() bool {
	return len(iw.ingress.Spec.TLS) > 0
}

func (iw *IngressWrapper) tryGetTLSHost() (string, bool) {
	if iw.supportsTLS() && len(iw.ingress.Spec.TLS[0].Hosts) > 0 {
		return iw.ingress.Spec.TLS[0].Hosts[0], true
	}
	return "", false
}

func (iw *IngressWrapper) rulesExist() bool {
	return len(iw.ingress.Spec.Rules) > 0
}

func (iw *IngressWrapper) tryGetRuleHost() (string, bool) {
	if iw.rulesExist() && iw.ingress.Spec.Rules[0].Host != "" {
		return iw.ingress.Spec.Rules[0].Host, true
	}
	return "", false
}

func (iw *IngressWrapper) statusLoadBalancerExist() bool {
	return len(iw.ingress.Status.LoadBalancer.Ingress) > 0
}

func (iw *IngressWrapper) tryGetStatusHost() (string, bool) {
	if iw.statusLoadBalancerExist() {
		ingressStatus := iw.ingress.Status.LoadBalancer.Ingress[0]
		if ingressStatus.Hostname != "" {
			return ingressStatus.Hostname, true
		} else if ingressStatus.IP != "" {
			return ingressStatus.IP, true
		}
	}
	return "", false
}

func (iw *IngressWrapper) getIngressSubPath() string {
	if iw.rulesExist() {
		rule := iw.ingress.Spec.Rules[0]
		if rule.HTTP != nil {
			if len(rule.HTTP.Paths) > 0 {
				return rule.HTTP.Paths[0].Path
			}
		}
	}
	return ""
}
