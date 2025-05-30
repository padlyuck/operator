/*


Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package operator

import (
	"context"
	"fmt"

	"github.com/go-logr/logr"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/predicate"

	vmv1beta1 "github.com/VictoriaMetrics/operator/api/operator/v1beta1"
	"github.com/VictoriaMetrics/operator/internal/config"
	"github.com/VictoriaMetrics/operator/internal/controller/operator/factory/k8stools"
	"github.com/VictoriaMetrics/operator/internal/controller/operator/factory/logger"
	"github.com/VictoriaMetrics/operator/internal/controller/operator/factory/vmagent"
)

// VMServiceScrapeReconciler reconciles a VMServiceScrape object
type VMServiceScrapeReconciler struct {
	client.Client
	Log          logr.Logger
	OriginScheme *runtime.Scheme
}

// Init implements crdController interface
func (r *VMServiceScrapeReconciler) Init(rclient client.Client, l logr.Logger, sc *runtime.Scheme, cf *config.BaseOperatorConf) {
	r.Client = rclient
	r.Log = l.WithName("controller.VMServiceScrape")
	r.OriginScheme = sc
}

// Scheme implements interface.
func (r *VMServiceScrapeReconciler) Scheme() *runtime.Scheme {
	return r.OriginScheme
}

// Reconcile general reconcile method for controller
// +kubebuilder:rbac:groups=operator.victoriametrics.com,resources=vmservicescrapes,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=operator.victoriametrics.com,resources=vmservicescrapes/status,verbs=get;update;patch
func (r *VMServiceScrapeReconciler) Reconcile(ctx context.Context, req ctrl.Request) (result ctrl.Result, err error) {
	instance := &vmv1beta1.VMServiceScrape{}
	reqLogger := r.Log.WithValues("vmservicescrape", req.Name, "namespace", req.Namespace)
	ctx = logger.AddToContext(ctx, reqLogger)
	defer func() {
		result, err = handleReconcileErrWithoutStatus(ctx, r.Client, instance, result, err)
	}()

	// Fetch the VMServiceScrape instance
	if err := r.Get(ctx, req.NamespacedName, instance); err != nil {
		return result, &getError{err, "vmservicescrape", req}
	}

	RegisterObjectStat(instance, "vmservicescrape")
	if instance.Spec.ParsingError != "" {
		return result, &parsingError{instance.Spec.ParsingError, "vmservicescrape"}
	}

	if vmAgentReconcileLimit.MustThrottleReconcile() {
		// fast path, rate limited
		return
	}

	vmAgentSync.Lock()
	defer vmAgentSync.Unlock()
	var objects vmv1beta1.VMAgentList
	if err := k8stools.ListObjectsByNamespace(ctx, r.Client, config.MustGetWatchNamespaces(), func(dst *vmv1beta1.VMAgentList) {
		objects.Items = append(objects.Items, dst.Items...)
	}); err != nil {
		return result, fmt.Errorf("cannot list vmauths for vmuser: %w", err)
	}

	for _, vmagentItem := range objects.Items {
		if !vmagentItem.DeletionTimestamp.IsZero() || vmagentItem.Spec.ParsingError != "" || vmagentItem.IsServiceScrapeUnmanaged() {
			continue
		}
		currentVMagent := &vmagentItem
		reqLogger := reqLogger.WithValues("vmagent", currentVMagent.Name, "parent_namespace", currentVMagent.Namespace)
		ctx := logger.AddToContext(ctx, reqLogger)
		if currentVMagent.Spec.DaemonSetMode {
			continue
		}
		// only check selector when deleting object,
		// since labels can be changed when updating and we can't tell if it was selected before, and we can't tell if it's creating or updating.
		if !instance.DeletionTimestamp.IsZero() {
			match, err := isSelectorsMatchesTargetCRD(ctx, r.Client, instance, currentVMagent, currentVMagent.Spec.ServiceScrapeSelector, currentVMagent.Spec.ServiceScrapeNamespaceSelector, currentVMagent.Spec.SelectAllByDefault)
			if err != nil {
				reqLogger.Error(err, "cannot match vmagent and vmServiceScrape")
				continue
			}
			if !match {
				continue
			}
		}

		if err := vmagent.CreateOrUpdateConfigurationSecret(ctx, r, currentVMagent, instance); err != nil {
			continue
		}
	}
	return
}

// SetupWithManager general setup method
func (r *VMServiceScrapeReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vmv1beta1.VMServiceScrape{}).
		WithEventFilter(predicate.TypedGenerationChangedPredicate[client.Object]{}).
		WithOptions(getDefaultOptions()).
		Complete(r)
}
