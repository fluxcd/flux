# Using GKE Workload Identity with Flux

When Flux is running in a GKE cluster with [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) enabled and you use Google Container Registry to host private images in your project, there are additional steps required for Flux to be able to check for updated images.

Without Workload Identity, Pods in the cluster by default assume the default IAM account of the GCP compute instances they are running on. With Workload Identity enabled, however, VM instance and Pod identity is completely separate. This results in Flux no longer being able to access a private GCR registry in the same project.

In this case, the Kubernetes service account as which Flux is running needs to be granted the Storage Object Viewer role to the registry's underlying GCS bucket to scan for updated images.

## Configure a GCP service account

The first step is to create an [IAM service account](https://cloud.google.com/docs/authentication/getting-started#creating_a_service_account) in the GCP project and assign it the Storage Object Viewer (`storage.objectViewer`) role in the GCS bucket that is backing the container registry in your project.

Next, the GCP service account needs to be assigned the `iam.workloadIdentityUser` role:

```bash
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:cluster_project.svc.id.goog[k8s_namespace/ksa_name]" \
  gsa_name@gsa_project.iam.gserviceaccount.com
```

So if your GCP project is called `total-mayhem-123456` and the GCP service account `flux-gcp` and Flux in your Kubernetes cluster(s) are running in the namespace `flux` and using the service account `flux` (the default), this would translate to the following:

```bash
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:total-mayhem-123456.svc.id.goog[flux/flux]" \
  flux-gcp@total-mayhem-123456.iam.gserviceaccount.com
```

## Configure K8s service account

In the second step you need to add an annotation to the Kubernetes service account as which the Flux pod is running in the cluster, so Workload Identity knows the relationship of GCP to K8s service account.

```yaml
apiVersion: v1
kind: ServiceAccount
metadata:
  labels:
    name: flux
  name: flux
  namespace: flux
  annotations:
    iam.gke.io/gcp-service-account=flux-gcp@total-mayhem-123456.iam.gserviceaccount.com
```

Alternatively, if you use the Helm chart to install Flux, you can set the annotations during installation:

```bash
# You need to escape the dots in the annotation key, else Helm will throw an error
helm upgrade -i flux fluxcd/flux \
--set serviceAccount.annotations.'iam\.gke\.io/gcp-service-account'='flux-gcp@total-mayhem-123456.iam.gserviceaccount.com'
--set [your other settings here] \
--namespace flux
```