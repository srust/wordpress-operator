package wordpress

import (
	"context"

	examplev1 "github.com/srust/wordpress-operator/pkg/apis/example/v1"
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
//   - pvc mysql
//   - pvc wordpress
//   - deployment mysql
//   - deployment wordpress
//   - service mysql (ClusterIP)
//   - service wordpress (LoadBalancer)
//   - mysql secret (mysql-pass)
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

	pvc := 

//   - pvc mysql
//   - pvc wordpress
//   - deployment mysql
//   - deployment wordpress
//   - service mysql (ClusterIP)
//   - service wordpress (LoadBalancer)
//   - mysql secret (mysql-pass)

	// Define a new Pod object
	pod := newPodForCR(instance)

	// Set Wordpress instance as the owner and controller
	if err := controllerutil.SetControllerReference(instance, pod, r.scheme); err != nil {
		return reconcile.Result{}, err
	}

	// Check if this Pod already exists
	found := &corev1.Pod{}
	err = r.client.Get(context.TODO(), types.NamespacedName{Name: pod.Name, Namespace: pod.Namespace}, found)
	if err != nil && errors.IsNotFound(err) {
		reqLogger.Info("Creating a new Pod", "Pod.Namespace", pod.Namespace, "Pod.Name", pod.Name)
		err = r.client.Create(context.TODO(), pod)
		if err != nil {
			return reconcile.Result{}, err
		}

		// Pod created successfully - don't requeue
		return reconcile.Result{}, nil
	} else if err != nil {
		return reconcile.Result{}, err
	}

	// Pod already exists - don't requeue
	reqLogger.Info("Skip reconcile: Pod already exists", "Pod.Namespace", found.Namespace, "Pod.Name", found.Name)
	return reconcile.Result{}, nil
}

// deploymentForMysql returns a mysql Deployment object
func (r *ReconcileMysql) deploymentForMysql(m *examplev1.Wordpress) *appsv1.Deployment {
	ls := labelsForMemcached(m.Name)
	password := m.Spec.SqlRootPassword

	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wordpress-mysql",
			Namespace: m.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: {
					app:  "wordpress",
					tier: "mysql",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: "wordpress-mysql",
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{{
						Image:   "mysql:5.6",
						Name:    "mysql",
						Ports: []corev1.ContainerPort{{
							ContainerPort: 3306,
							Name:          "mysql",
						}},
					}},
				},
			},
		},
	}

	// Set Wordpress instance as the owner of the Deployment.
	controllerutil.SetControllerReference(m, dep, r.scheme)
	return dep
}

// serviceForMysql function takes in a Wordpress object and returns a Service for that object.
func (r *ReconcileMysql) serviceForMysql(m *examplev1.Wordpress) *corev1.Service {
	ser := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "wordpress-mysql",
			Namespace: m.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Selector: {
				app:  "wordpress",
				tier: "mysql",
			},
			Ports: []corev1.ServicePort{
				{
					Port: 3306,
					Name: m.Name,
				},
			},
			ClusterIp: "None",
		},
	}
	// Set Memcached instance as the owner of the Service.
	controllerutil.SetControllerReference(m, ser, r.scheme)
	return ser
}
