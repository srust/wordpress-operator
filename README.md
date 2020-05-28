# wordpress-operator
A basic Wordpress Operator.

# Build

Type `make` to build the operator. Assumes *operator-sdk* is in the path and *docker* is available.

```
make
```

By default the image is built under quay.io/srust. If you need a different image name or version you can specify them:

```
IMAGE=quay.io/user/wordpress-operator VERSION=1.0 make
```

# Upload the operator image

Push the image to the remote repository for deployment.

```
make push
```

```
IMAGE=quay.io/user/wordpress-operator VERSION=1.0 make push
```

**NOTE**: assumes logged in to remote registry

# Deploy the Wordpress CRD

```
kubectl create -f deploy/crds/example.com_wordpresses_crd.yaml
```

# Deploy the Wordpress Operator

```
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/operator.yaml
```

# Deploy Wordpress

```
kubectl create -f wordpress.yaml
```

# Verify Deployment

```
$ kubectl get deployment,service,pvc,secret
NAME                                 READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/wordpress            1/1     1            1           7s
deployment.apps/wordpress-mysql      1/1     1            1           7s
deployment.apps/wordpress-operator   1/1     1            1           11s

NAME                                 TYPE           CLUSTER-IP     EXTERNAL-IP   PORT(S)             AGE
service/kubernetes                   ClusterIP      10.96.0.1      <none>        443/TCP             3h2m
service/wordpress                    LoadBalancer   10.110.8.241   <pending>     80:30317/TCP        7s
service/wordpress-mysql              ClusterIP      None           <none>        3306/TCP            7s
service/wordpress-operator-metrics   ClusterIP      10.98.48.23    <none>        8383/TCP,8686/TCP   8s

NAME                                   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
persistentvolumeclaim/mysql-pv-claim   Bound    pvc-2127fc52-40bf-4e58-9440-7fa5fd18e59d   20Gi       RWO            standard       7s
persistentvolumeclaim/wp-pv-claim      Bound    pvc-17a89d6f-50b4-4ee1-a974-da990717e233   20Gi       RWO            standard       7s

NAME                                    TYPE                                  DATA   AGE
secret/default-token-dhm62              kubernetes.io/service-account-token   3      3h2m
secret/mysql-pass                       Opaque                                1      7s
secret/wordpress-operator-token-bqv54   kubernetes.io/service-account-token   3      11s
```

# NOTES

* PVCs are not removed when "Wordpress" is removed. It is intended that the data is retained for future use, unless deleted by an administrator.
* On "minikube" the EXTERNAL-IP for the LoadBalancer will stay **\<pending\>**
* Much of the configuration / naming of services is hard-coded in the Operator, making this a single deployment example only. This could be improved to support multiple deployments, perhaps through ConfigMap.
* The mysql password is also plaintext in the yaml. This could be improved to be stored in a secret directly.
