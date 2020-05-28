package wordpress

import (
	"context"
	"time"

	examplev1 "github.com/srust/wordpress-operator/pkg/apis/example/v1"
	resource "k8s.io/apimachinery/pkg/api/resource"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_wordpress")

// Add creates a new Wordpress Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileWordpress{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("wordpress-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Wordpress
	err = c.Watch(&source.Kind{Type: &examplev1.Wordpress{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to deployment for mysql and wordpress
	err = c.Watch(&source.Kind{Type: &appsv1.Deployment{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &examplev1.Wordpress{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to services for mysql and wordpress
	err = c.Watch(&source.Kind{Type: &corev1.Service{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &examplev1.Wordpress{},
	})
	if err != nil {
		return err
	}

	// Watch for changes to PersistentVolumeClaims for mysql and wordpress
	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &examplev1.Wordpress{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileWordpress implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileWordpress{}

// ReconcileWordpress reconciles a Wordpress object
type ReconcileWordpress struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a Wordpress object and makes changes based on the state read
// and what is in the Wordpress.Spec
//
// We need to reconcile:
//   - mysql secret (mysql-pass)
//   - pvc mysql
//   - pvc wordpress
//   - deployment mysql
//   - deployment wordpress
//   - service mysql (ClusterIP)
//   - service wordpress (LoadBalancer)
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileWordpress) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Wordpress")

	// Fetch the Wordpress instance
	instance := &examplev1.Wordpress{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	// secret
	secret := r.MysqlSecret(instance)

	foundsecret := &corev1.Secret{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: secret.Name, Namespace: secret.Namespace}, foundsecret)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new SECRET", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
		err = r.client.Create(context.TODO(), secret)
		if err != nil {
			reqLogger.Error(err, "Failed to create new SECRET.", "Secret.Namespace", secret.Namespace, "Secret.Name", secret.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// pvc mysql
	pvc := r.MysqlPvc(instance)

	// idempotent: check if this already exists
	foundpvc := &corev1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, foundpvc)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new PVC", "Pvc.Namespace", pvc.Namespace, "Pvc.Name", pvc.Name)
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new PVC.", "Pvc.Namespace", pvc.Namespace, "Pvc.Name", pvc.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// - pvc wordpress
	pvc = r.WordpressPvc(instance)

	// idempotent: check if this already exists
	foundpvc = &corev1.PersistentVolumeClaim{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pvc.Name, Namespace: pvc.Namespace}, foundpvc)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new PVC", "Pvc.Namespace", pvc.Namespace, "Pvc.Name", pvc.Name)
		err = r.client.Create(context.TODO(), pvc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new PVC.", "Pvc.Namespace", pvc.Namespace, "Pvc.Name", pvc.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// - deployment mysql
	deployment := r.MysqlDeployment(instance)

	// idempotent: check if this already exists
	founddep := &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, founddep)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DEPLOYMENT", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		err = r.client.Create(context.TODO(), deployment)
		if err != nil {
			reqLogger.Error(err, "Failed to create new DEPLOYMENT.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// - service mysql (ClusterIP)
	svc := r.MysqlService(instance)

	// idempotent: check if this already exists
	foundsvc := &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, foundsvc)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new SERVICE", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new SERVICE.", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// - deployment wordpress
	deployment = r.WordpressDeployment(instance)

	// idempotent: check if this already exists
	founddep = &appsv1.Deployment{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, founddep)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new DEPLOYMENT", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
		err = r.client.Create(context.TODO(), deployment)
		if err != nil {
			reqLogger.Error(err, "Failed to create new DEPLOYMENT.", "Deployment.Namespace", deployment.Namespace, "Deployment.Name", deployment.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// - service wordpress (LoadBalancer)
	svc = r.WordpressService(instance)

	// idempotent: check if this already exists
	foundsvc = &corev1.Service{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: deployment.Name, Namespace: deployment.Namespace}, foundsvc)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new SERVICE", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
		err = r.client.Create(context.TODO(), svc)
		if err != nil {
			reqLogger.Error(err, "Failed to create new SERVICE.", "Service.Namespace", svc.Namespace, "Service.Name", svc.Name)
			return reconcile.Result{}, err
		}
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Reconcile for any reason than error after 5 seconds
	return reconcile.Result{RequeueAfter: time.Second*5}, nil
}

func (r* ReconcileWordpress) MysqlSecret(w *examplev1.Wordpress) *corev1.Secret {
	name     := "mysql-pass"
	password := w.Spec.SqlRootPassword

	secret := &corev1.Secret {
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: w.Namespace,
		},
		Type: "Opaque",
		StringData: map[string]string {
			"password": password,
		},
	}

	controllerutil.SetControllerReference(w, secret, r.scheme)
	return secret
}

func (r* ReconcileWordpress) MysqlPvc(w *examplev1.Wordpress) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-pv-claim",
			Namespace: w.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{ corev1.ReadWriteOnce },
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
	}

	return pvc
}

func (r* ReconcileWordpress) WordpressPvc(w *examplev1.Wordpress) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wp-pv-claim",
			Namespace: w.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{ corev1.ReadWriteOnce },
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse("20Gi"),
				},
			},
		},
	}

	return pvc
}

// deploymentForMysql returns a mysql Deployment object
func (r *ReconcileWordpress) MysqlDeployment(w *examplev1.Wordpress) *appsv1.Deployment {
	labels := map[string]string {
		"app":  "wordpress",
	}
	matchlabels := map[string]string {
		"app":  "wordpress",
		"tier": "mysql",
	}

	rootPasswordSecret := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "mysql-pass"},
			Key: "password",
		},
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wordpress-mysql",
			Namespace: w.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchlabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: matchlabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "mysql:5.6",
						Name:    "mysql",
						Env: []corev1.EnvVar{{
							Name: "MYSQL_ROOT_PASSWORD",
							ValueFrom: rootPasswordSecret,
						}},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 3306,
							Name:          "mysql",
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "mysql-persistent-storage",
							MountPath: "/var/lib/mysql",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "mysql-persistent-storage",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "mysql-pv-claim",
							},
						},
					}},
				},
			},
		},
	}

	// Set Wordpress instance as the owner of the Deployment.
	controllerutil.SetControllerReference(w, dep, r.scheme)
	return dep
}

// returns a Wordpress Deployment object
func (r *ReconcileWordpress) WordpressDeployment(w *examplev1.Wordpress) *appsv1.Deployment {
	labels := map[string]string {
		"app":  "wordpress",
	}
	matchlabels := map[string]string {
		"app":  "wordpress",
		"tier": "frontend",
	}

	rootPasswordSecret := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: "mysql-pass"},
			Key: "password",
		},
	}

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wordpress",
			Namespace: w.Namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: matchlabels,
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: matchlabels,
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "wordpress:4.8-apache",
						Name:    "wordpress",
						Env: []corev1.EnvVar{
							{
								Name: "WORDPRESS_DB_HOST",
								Value: "wordpress-mysql",
							},
							{
								Name: "WORDPRESS_DB_PASSWORD",
								ValueFrom: rootPasswordSecret,
							},
						},
						Ports: []corev1.ContainerPort{{
							ContainerPort: 80,
							Name:          "wordpress",
						}},
						VolumeMounts: []corev1.VolumeMount{{
							Name:      "wordpress-persistent-storage",
							MountPath: "/var/www/html",
						}},
					}},
					Volumes: []corev1.Volume{{
						Name: "wordpress-persistent-storage",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "wp-pv-claim",
							},
						},
					}},
				},
			},
		},
	}

	// Set Wordpress instance as the owner of the Deployment.
	controllerutil.SetControllerReference(w, dep, r.scheme)
	return dep
}

// serviceForMysql function takes in a Wordpress object and returns a Service for that object.
func (r* ReconcileWordpress) MysqlService(w *examplev1.Wordpress) *corev1.Service {
	selector := map[string]string {
		"app":  "wordpress",
		"tier": "mysql",
	}
	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wordpress-mysql",
			Namespace: w.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{
				{
					Port: 3306,
					Name: "mysql",
				},
			},
			ClusterIP: "None",
		},
	}

	// Set Wordpress instance as the owner of the Service.
	controllerutil.SetControllerReference(w, ser, r.scheme)
	return ser
}

// serviceForMysql function takes in a Wordpress object and returns a Service for that object.
func (r* ReconcileWordpress) WordpressService(w *examplev1.Wordpress) *corev1.Service {
	selector := map[string]string {
		"app":  "wordpress",
		"tier": "frontend",
	}
	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wordpress",
			Namespace: w.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: selector,
			Ports: []corev1.ServicePort{
				{
					Port: 80,
					Name: "wordpress",
				},
			},
			Type: "LoadBalancer",
		},
	}

	// Set Wordpress instance as the owner of the Service.
	controllerutil.SetControllerReference(w, ser, r.scheme)
	return ser
}
