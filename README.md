## Deprecation notice
This project has been deprecated. Check out [k8sgpt-operator](https://github.com/k8sgpt-ai/k8sgpt-operator) instead.

# Shepard
Wraps k8sgpt functionality with the ClusterAnalysis CR for declarative, Kubernetes-native cluster health analyses.

Free, in-cluster analysis is accomplished via k8sgpt's `localai` backend.

## Requirements
The following services must be pre-installed in your cluster:
- [k8sgpt](https://github.com/k8sgpt-ai/k8sgpt)
- [local-ai](https://github.com/go-skynet/LocalAI)
