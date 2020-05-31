# wordpress-operator
A basic Wordpress Operator.

This operator deploys a wordpress service backed by mysql, with a LoadBalancer
service for Wordpress, a NodePort for Mysql, backed by two PVCs, one for each
service deployment.

It is optimized for minikube, and expects a *default* StorageClass to exist.

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

Push the image to the remote registry for deployment.

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

# Operator Configuration

The Wordpress Operator takes the following configuration options as environment variables in the deployment for the Operator, change by editing `deploy/operator.yaml`

| env | default | desc
------| --------| --------
| WORDPRESS_SECRET_NAME | mysql-pass | The name of the secret created by the operator where the mysql root password is stored |
| WORDPRESS_SECRET_KEY  | password   | The secret key created by the operator to store the mysql root password |
| WORDPRESS_PVC_SIZE    | 20Gi       | PVC size for the mysql and wordpress backing PVCs |
| WORDPRESS_IMAGE_MYSQL | mysql:5.6  | mysql image to use |
| WORDPRESS_IMAGE_WORDPRESS | wordpress:4.8-apache | wordpress image to use |

# Deploy the Wordpress Operator

```
kubectl create -f deploy/role.yaml
kubectl create -f deploy/role_binding.yaml
kubectl create -f deploy/service_account.yaml
kubectl create -f deploy/operator.yaml
```

# Wordpress Configuration

Edit `wordpress.yaml` to configure the `sqlRootPassword` and the `retainVolumes` setting. The `retainVolumes` setting defaults to false, which means PVCs will be deleted when the wordpress deployment is deleted. Set `retainVolumes` to `true` to keep PVCs around.

```
apiVersion: example.com/v1
kind: Wordpress
metadata:
  name: mysite
spec:
  sqlRootPassword: plaintextpassword
  retainVolumes: false
```

# Deploy Wordpress Instance

```
kubectl create -f wordpress.yaml
```

# Minikube

Use `minikube tunnel` to expose an EXTERNAL-IP for the wordpress load balancer.

```
$ minikube tunnel
```

# Verify Deployment

```
$ kubectl get deployment,service,pvc,secret
NAME                                 READY   UP-TO-DATE   AVAILABLE   AGE
deployment.apps/wordpress            1/1     1            1           11s
deployment.apps/wordpress-mysql      1/1     1            1           11s
deployment.apps/wordpress-operator   1/1     1            1           17s

NAME                                 TYPE           CLUSTER-IP       EXTERNAL-IP      PORT(S)             AGE
service/kubernetes                   ClusterIP      10.96.0.1        <none>           443/TCP             3d22h
service/wordpress                    LoadBalancer   10.111.173.251   10.111.173.251   80:31963/TCP        11s
service/wordpress-mysql              ClusterIP      None             <none>           3306/TCP            11s
service/wordpress-operator-metrics   ClusterIP      10.106.234.30    <none>           8383/TCP,8686/TCP   12s

NAME                                   STATUS   VOLUME                                     CAPACITY   ACCESS MODES   STORAGECLASS   AGE
persistentvolumeclaim/mysql-pv-claim   Bound    pvc-d390d20c-f217-43f2-b8e6-2d15b4a41f8c   20Gi       RWO            standard       12s
persistentvolumeclaim/wp-pv-claim      Bound    pvc-b3110005-25f1-4926-8288-e7b059e1cd82   20Gi       RWO            standard       11s

NAME                                    TYPE                                  DATA   AGE
secret/default-token-dhm62              kubernetes.io/service-account-token   3      3d22h
secret/mysql-pass                       Opaque                                1      12s
secret/wordpress-operator-token-lh6pn   kubernetes.io/service-account-token   3      17s
```

# Verify Wordpress Instance Conditions

Check Status Conditions in the wordpress instance to ensure that each component was created

```
$ kubectl get wordpress/mysite -o yaml
status:
  conditions:
  - lastTransitionTime: "2020-05-31T22:53:59Z"
    message: mysqlDeployment has been created
    reason: operatorCreated
    status: "True"
    type: mysqlDeploymentCreated
  - lastTransitionTime: "2020-05-31T22:53:58Z"
    message: mysqlPVC has been created
    reason: operatorCreated
    status: "True"
    type: mysqlPVCCreated
  - lastTransitionTime: "2020-05-31T22:53:59Z"
    message: mysqlService has been created
    reason: operatorCreated
    status: "True"
    type: mysqlServiceCreated
  - lastTransitionTime: "2020-05-31T22:53:58Z"
    message: secret has been created
    reason: operatorCreated
    status: "True"
    type: secretCreated
  - lastTransitionTime: "2020-05-31T22:53:59Z"
    message: wordpressDeployment has been created
    reason: operatorCreated
    status: "True"
    type: wordpressDeploymentCreated
  - lastTransitionTime: "2020-05-31T22:53:59Z"
    message: wordpressPVC has been created
    reason: operatorCreated
    status: "True"
    type: wordpressPVCCreated
  - lastTransitionTime: "2020-05-31T22:53:59Z"
    message: wordpressService has been created
    reason: operatorCreated
    status: "True"
    type: wordpressServiceCreated
```

# NOTES

* Edit the operator deployment to update image versions for mysql and wordpress if desired
* One instance of wordpress is currently supported at the same time, as various service and PVC names are well-known names, and statically defined.
* The mysql password is also plaintext in the yaml. This could be improved to be stored in a secret directly.
