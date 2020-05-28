kubectl delete -f wordpress.yaml
kubectl delete -f deploy/operator.yaml
kubectl delete -f deploy/role.yaml
kubectl delete -f deploy/role_binding.yaml
kubectl delete -f deploy/service_account.yaml
kubectl delete pvc/wp-pv-claim
kubectl delete pvc/mysql-pv-claim
