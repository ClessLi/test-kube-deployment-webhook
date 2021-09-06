/*
Copyright 2021.

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

package webhook

import (
	"context"
	"fmt"
	corev1 "k8s.io/api/apps/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/json"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var deploymentLog = logf.Log.WithName("deployment-resource")

const (
	mutatedAnnotation = "test-kube-deployment-webhook-mutated"
	isMutated         = "isMutated"
)

//+kubebuilder:webhook:path=/mutate-v1-deployment,mutating=true,failurePolicy=fail,groups=apps,resources=deployments,verbs=create;update,versions=v1,name=mdeployment.kb.io

type deploymentAnnotator struct {
	Client  client.Client
	decoder *admission.Decoder
}

func (d *deploymentAnnotator) Handle(ctx context.Context, request admission.Request) admission.Response {
	dm := &corev1.Deployment{}
	deploymentLog.Info("decode", "name", request.Name)
	err := d.decoder.Decode(request, dm)
	if err != nil {
		deploymentLog.Error(err, "failed to decode", "name", request.Name)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// mutate the fields in deployment
	if dm.Annotations == nil {
		dm.Annotations = make(map[string]string)
	}

	if av, ok := dm.Annotations[mutatedAnnotation]; ok && av == isMutated {
		deploymentLog.Info("pass mutate", "name", dm.Name)
		return admission.Allowed("it has been mutated by test-kube-deployment-webhook")
	}

	dm.Spec.Template.Spec.InitContainers = append(dm.Spec.Template.Spec.InitContainers, v1.Container{
		Name:  "initial-test",
		Image: "busybox",
		Args:  []string{"/bin/sh", "-c", "date; echo Test for initial pod; echo deploy name is $DM_NAME"},
		Env: []v1.EnvVar{{
			Name:  "DM_NAME",
			Value: fmt.Sprintf("%s/%s/%s_%s", dm.GroupVersionKind().Group, dm.APIVersion, dm.Kind, dm.Name),
		}},
	})

	marshaledDM, err := json.Marshal(dm)
	if err != nil {
		deploymentLog.Error(err, "failed to marshal mutated deployment", "name", dm.Name)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(request.Object.Raw, marshaledDM)
}

func (d *deploymentAnnotator) InjectDecoder(decoder *admission.Decoder) error {
	d.decoder = decoder
	return nil
}

func RegisterWebhook(mgr manager.Manager) {
	mgr.GetWebhookServer().Register("/mutate-v1-deployment", &webhook.Admission{Handler: &deploymentAnnotator{Client: mgr.GetClient()}})
}
