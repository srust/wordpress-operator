kubectl create -f deploy/crds/example.com_wordpresses_crd.yaml
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/operator.yaml
kubectl create -f wordpress.yaml
