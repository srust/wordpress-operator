package wordpress

import (
	"context"
	"os"
	"fmt"

	examplev1 "github.com/srust/wordpress-operator/pkg/apis/example/v1"
	condv1 "github.com/operator-framework/operator-sdk/pkg/status"

	resource "k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/api/meta"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_wordpress")

const wordpressFinalizer = "wordpress.example.com"

func contains(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
}

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
	client  client.Client
	scheme *runtime.Scheme
	logger  logr.Logger
}

// Reconcile reads that state of the cluster for a Wordpress object and makes
// changes based on the state read and what is in the Wordpress.Spec
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
	r.logger = log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	r.logger.Info("Reconciling Wordpress")

	// Fetch the Wordpress instance
	instance, err := r.findWordpressInstance(&request)
	if err != nil {
		if errors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// reconcile secret
	err = r.reconcileSecret(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.updateStatus(instance, "secret")

	// reconcile PVC for Mysql
	err = r.reconcileMysqlPVC(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.updateStatus(instance, "mysqlPVC")

	// reconcile PVC for Wordpress
	err = r.reconcileWordpressPVC(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.updateStatus(instance, "wordpressPVC")

	// reconcile deployment for Mysql
	err = r.reconcileMysqlDeployment(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.updateStatus(instance, "mysqlDeployment")

	// reconcile service for Mysql
	err = r.reconcileMysqlService(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.updateStatus(instance, "mysqlService")

	// reconcile deployment for Wordpress
	err = r.reconcileWordpressDeployment(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.updateStatus(instance, "wordpressDeployment")

	// reconcile service for Wordpress
	err = r.reconcileWordpressService(instance)
	if err != nil {
		return reconcile.Result{}, err
	}
	r.updateStatus(instance, "wordpressService")

	// Check if Wordpress instance is marked to be deleted
	isWordpressMarkedToBeDeleted := instance.GetDeletionTimestamp() != nil
	if isWordpressMarkedToBeDeleted {
		if contains(instance.GetFinalizers(), wordpressFinalizer) {
			// Run finalization logic for wordpressFinalizer. If the
			// finalization logic fails, don't remove the finalizer so
			// that we can retry during the next reconciliation.
			if err := r.finalizeWordpress(instance); err != nil {
				return reconcile.Result{}, err
			}

			// Remove wordpressFinalizer. Once all finalizers have been
			// removed, the object will be deleted.
			controllerutil.RemoveFinalizer(instance, wordpressFinalizer)
			err := r.client.Update(context.TODO(), instance)
			if err != nil {
				return reconcile.Result{}, err
			}
		}
		return reconcile.Result{}, nil
	}

	// Add finalizer for this CR
	if !contains(instance.GetFinalizers(), wordpressFinalizer) {
		if err := r.addFinalizer(instance); err != nil {
			return reconcile.Result{}, err
		}
	}

	return reconcile.Result{}, nil
}

// find and return Wordpress instance
func (r* ReconcileWordpress) findWordpressInstance(request *reconcile.Request) (*examplev1.Wordpress, error) {
	instance := &examplev1.Wordpress{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)

	return instance, err
}

/////////////////////////////////////////////////////////////////////
// CreateObject creates an object
/////////////////////////////////////////////////////////////////////
func (r* ReconcileWordpress) CreateObject(obj runtime.Object, kind string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		r.logger.Error(err, "failed to get meta information", "Kind", kind)
		return err
	}

	err = r.client.Create(context.TODO(), obj)
	if err == nil {
		r.logger.Info("created object", "Kind", kind, "Name", accessor.GetName())
	} else {
		if errors.IsAlreadyExists(err) {
			return nil
		}
		r.logger.Error(err, "failed to create object", "Kind", kind, "Name", accessor.GetName())
		return err
	}

	return nil
}

/////////////////////////////////////////////////////////////////////
// DeleteObject deletes an object
/////////////////////////////////////////////////////////////////////
func (r* ReconcileWordpress) DeleteObject(obj runtime.Object, kind string) error {
	accessor, err := meta.Accessor(obj)
	if err != nil {
		r.logger.Error(err, "failed to get meta information", "Kind", kind)
		return err
	}

	err = r.client.Delete(context.TODO(), obj)
	if err == nil {
		r.logger.Info("deleted object", "Kind", kind, "Name", accessor.GetName())
		return nil
	} else if errors.IsNotFound(err) {
		return nil
	} else {
		r.logger.Error(err, "failed to delete object", "Kind", kind, "Name", accessor.GetName())
		return err;
	}
}

/////////////////////////////////////////////////////////////////////
// Reconcile Secret
/////////////////////////////////////////////////////////////////////

// return mysql secret object
func (r* ReconcileWordpress) genMysqlSecret(w *examplev1.Wordpress) *corev1.Secret {
	secretName := os.Getenv("WORDPRESS_SECRET_NAME")
	secretKey  := os.Getenv("WORDPRESS_SECRET_KEY")

	name     := secretName
	password := w.Spec.SqlRootPassword

	secret := &corev1.Secret {
	    ObjectMeta: metav1.ObjectMeta{
		Name:      name,
		Namespace: w.Namespace,
	    },
	    Type: "Opaque",
	    StringData: map[string]string {
		secretKey: password,
	    },
	}

	controllerutil.SetControllerReference(w, secret, r.scheme)
	return secret
}

// create or update mysql secret
func (r* ReconcileWordpress) reconcileSecret(w *examplev1.Wordpress) (error) {
	// secret
	secret := r.genMysqlSecret(w)

	// create or update secret
	err := r.CreateObject(secret, "Secret")
	return err;
}

/////////////////////////////////////////////////////////////////////
// Reconcile Mysql PVC
/////////////////////////////////////////////////////////////////////

// return mysql PVC object
func (r* ReconcileWordpress) genMysqlPVC(w *examplev1.Wordpress) *corev1.PersistentVolumeClaim {
	pvcSize := os.Getenv("WORDPRESS_PVC_SIZE")

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysql-pv-claim",
			Namespace: w.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{ corev1.ReadWriteOnce },
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(pvcSize),
				},
			},
		},
	}

	return pvc
}

// create or update mysql PVC object
func (r* ReconcileWordpress) reconcileMysqlPVC(w *examplev1.Wordpress) (error) {
	pvc := r.genMysqlPVC(w)

	// create PVC
	err := r.CreateObject(pvc, "PersistentVolumeClaim")
	return err
}

/////////////////////////////////////////////////////////////////////
// Reconcile Wordpress PVC
/////////////////////////////////////////////////////////////////////

// return wordpress PVC object
func (r* ReconcileWordpress) genWordpressPVC(w *examplev1.Wordpress) *corev1.PersistentVolumeClaim {
	pvcSize := os.Getenv("WORDPRESS_PVC_SIZE")

	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wp-pv-claim",
			Namespace: w.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{ corev1.ReadWriteOnce },
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{
					corev1.ResourceStorage: resource.MustParse(pvcSize),
				},
			},
		},
	}

	return pvc
}

// create or update wordpress PVC
func (r* ReconcileWordpress) reconcileWordpressPVC(w *examplev1.Wordpress) (error) {
	pvc := r.genWordpressPVC(w)

	// create PVC
	err := r.CreateObject(pvc, "PersistentVolumeClaim")
	return err
}

/////////////////////////////////////////////////////////////////////
// Reconcile Mysql Deployment
/////////////////////////////////////////////////////////////////////

func (r* ReconcileWordpress) genRootPasswordSecret() *corev1.EnvVarSource {
	secretName := os.Getenv("WORDPRESS_SECRET_NAME")
	secretKey  := os.Getenv("WORDPRESS_SECRET_KEY")

	envvar := &corev1.EnvVarSource{
		SecretKeyRef: &corev1.SecretKeySelector{
			LocalObjectReference: corev1.LocalObjectReference{Name: secretName},
			Key: secretKey,
		},
	}

	return envvar
}

// return mysql deployment object
func (r *ReconcileWordpress) genMysqlDeployment(w *examplev1.Wordpress) *appsv1.Deployment {
	labels := map[string]string {
		"app":  "wordpress",
	}
	matchlabels := map[string]string {
		"app":  "wordpress",
		"tier": "mysql",
	}

	imageName  := os.Getenv("WORDPRESS_IMAGE_MYSQL")
	rootPasswordSecret := r.genRootPasswordSecret()

	deployment := &appsv1.Deployment{
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
						Image:   imageName,
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
	controllerutil.SetControllerReference(w, deployment, r.scheme)
	return deployment
}

// create or update mysql deployment
func (r* ReconcileWordpress) reconcileMysqlDeployment(w *examplev1.Wordpress) (error) {
	deployment := r.genMysqlDeployment(w)

	// create Deployment
	err := r.CreateObject(deployment, "Deployment")
	return err
}

/////////////////////////////////////////////////////////////////////
// Reconcile Mysql Deployment
/////////////////////////////////////////////////////////////////////

// returns a Wordpress Deployment object
func (r *ReconcileWordpress) genWordpressDeployment(w *examplev1.Wordpress) *appsv1.Deployment {
	labels := map[string]string {
		"app":  "wordpress",
	}
	matchlabels := map[string]string {
		"app":  "wordpress",
		"tier": "frontend",
	}

	imageName  := os.Getenv("WORDPRESS_IMAGE_WORDPRESS")
	rootPasswordSecret := r.genRootPasswordSecret()

	deployment := &appsv1.Deployment{
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
						Image:   imageName,
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
	controllerutil.SetControllerReference(w, deployment, r.scheme)
	return deployment
}

// Creates or Updates a Wordpress Deployment object
func (r* ReconcileWordpress) reconcileWordpressDeployment(w *examplev1.Wordpress) (error) {
	deployment := r.genWordpressDeployment(w)

	// create Wordpress Deployment
	err := r.CreateObject(deployment, "Deployment")
	return err
}

/////////////////////////////////////////////////////////////////////
// Reconcile Mysql Service
/////////////////////////////////////////////////////////////////////

// returns a mysql service object
func (r* ReconcileWordpress) genMysqlService(w *examplev1.Wordpress) *corev1.Service {
	selector := map[string]string {
		"app":  "wordpress",
		"tier": "mysql",
	}
	service := &corev1.Service{
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
	controllerutil.SetControllerReference(w, service, r.scheme)
	return service
}

// Creates or Updates a Mysql Service object
func (r* ReconcileWordpress) reconcileMysqlService(w *examplev1.Wordpress) (error) {
	service := r.genMysqlService(w)

	// create Mysql Service
	err := r.CreateObject(service, "Service")
	return err
}

/////////////////////////////////////////////////////////////////////
// Reconcile Wordpress Service
/////////////////////////////////////////////////////////////////////

// create or update wordpress service
func (r* ReconcileWordpress) genWordpressService(w *examplev1.Wordpress) *corev1.Service {
	selector := map[string]string {
		"app":  "wordpress",
		"tier": "frontend",
	}
	service := &corev1.Service{
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
	controllerutil.SetControllerReference(w, service, r.scheme)
	return service
}

// Creates or Updates a Wordpress Service object
func (r* ReconcileWordpress) reconcileWordpressService(w *examplev1.Wordpress) (error) {
	service := r.genWordpressService(w)

	// create Mysql Service
	err := r.CreateObject(service, "Service")
	return err
}

/////////////////////////////////////////////////////////////////////
// Wordpress Finalizer
/////////////////////////////////////////////////////////////////////

// add finalizer for Wordpress
func (r *ReconcileWordpress) addFinalizer(w *examplev1.Wordpress) error {
	r.logger.Info("Adding Finalizer for Wordpress")
	controllerutil.AddFinalizer(w, wordpressFinalizer)

	// Update CR
	err := r.client.Update(context.TODO(), w)
	if err != nil {
		r.logger.Error(err, "Failed to update Wordpress with finalizer")
		return err
	}
	return nil
}

func (r *ReconcileWordpress) finalizeMysqlPVC(w *examplev1.Wordpress) error {
	pvc := r.genMysqlPVC(w)

	// delete PVC
	err := r.DeleteObject(pvc, "PersistentVolumeClaim")
	return err
}

func (r *ReconcileWordpress) finalizeWordpressPVC(w *examplev1.Wordpress) error {
	pvc := r.genWordpressPVC(w)

	// delete PVC
	err := r.DeleteObject(pvc, "PersistentVolumeClaim")
	return err
}

// run finalizer for Wordpress
func (r *ReconcileWordpress) finalizeWordpress(w *examplev1.Wordpress) error {
	// delete PVCs if not retaining
	if w.Spec.RetainVolumes == false {
		// delete mysql PVC
		err := r.finalizeMysqlPVC(w)
		if err != nil {
			return err
		}

		// delete wordpress PVC
		err = r.finalizeWordpressPVC(w)
		if err != nil {
			return err
		}
	}
	r.logger.Info("Successfully finalized Wordpress")
	return nil
}

/////////////////////////////////////////////////////////////////////
// Wordpress Status
/////////////////////////////////////////////////////////////////////
func (r *ReconcileWordpress) updateStatus(w *examplev1.Wordpress, field string) {
    condtype := fmt.Sprintf("%sCreated", field)
    reason   := "operatorCreated"
    message  := fmt.Sprintf("%s has been created", field)

    cond := condv1.Condition {
	Type:               condv1.ConditionType(condtype),
	Status:             corev1.ConditionTrue,
	Reason:             condv1.ConditionReason(reason),
	Message:            message,
	LastTransitionTime: metav1.Now(),
    }

    if ok := w.Status.Conditions.SetCondition(cond); ok {
	err := r.client.Status().Update(context.TODO(), w)
	if err != nil {
	    r.logger.Error(err, "Failed to update wordpress Status")
	}
    }
}
