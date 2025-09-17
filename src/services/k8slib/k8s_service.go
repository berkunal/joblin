package k8slib

import (
	"context"
	"fmt"
	"io"
	"time"

	"github.com/berkunal/joblin/src/models"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
)

const (
	DefaultPythonImage    = "python:3.11-slim"
	JoblinManagedLabel    = "app.kubernetes.io/managed-by"
	JoblinNameLabel       = "app.kubernetes.io/name"
	JoblinInstanceLabel   = "app.kubernetes.io/instance"
	JoblinManagedValue    = "joblin"
	PythonScriptContainer = "python-script"
)

type K8sService struct {
	clientset *kubernetes.Clientset
	config    *rest.Config
}

func NewK8sService(kubeconfig string, context string) (*K8sService, error) {
	config, err := buildConfig(kubeconfig, context)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubernetes config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &K8sService{
		clientset: clientset,
		config:    config,
	}, nil
}

func NewK8sServiceFromCluster() (*K8sService, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build in-cluster config: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	return &K8sService{
		clientset: clientset,
		config:    config,
	}, nil
}

func buildConfig(kubeconfig string, context string) (*rest.Config, error) {
	if kubeconfig == "" {
		if home := homedir.HomeDir(); home != "" {
			kubeconfig = filepath.Join(home, ".kube", "config")
		}
	}

	configLoader := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(
		&clientcmd.ClientConfigLoadingRules{ExplicitPath: kubeconfig},
		&clientcmd.ConfigOverrides{CurrentContext: context},
	)

	config, err := configLoader.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load kubeconfig: %w", err)
	}

	return config, nil
}

func (k *K8sService) CreateJob(ctx context.Context, job *models.Job) error {
	if err := job.Validate(); err != nil {
		return fmt.Errorf("invalid job: %w", err)
	}

	configMap, err := k.createScriptConfigMap(ctx, job)
	if err != nil {
		return fmt.Errorf("failed to create script configmap: %w", err)
	}

	k8sJob, err := k.buildKubernetesJob(job, configMap.Name)
	if err != nil {
		return fmt.Errorf("failed to build kubernetes job: %w", err)
	}

	_, err = k.clientset.BatchV1().Jobs(job.Namespace).Create(ctx, k8sJob, metav1.CreateOptions{})
	if err != nil {
		k.clientset.CoreV1().ConfigMaps(job.Namespace).Delete(ctx, configMap.Name, metav1.DeleteOptions{})
		return fmt.Errorf("failed to create kubernetes job: %w", err)
	}

	return nil
}

func (k *K8sService) createScriptConfigMap(ctx context.Context, job *models.Job) (*corev1.ConfigMap, error) {
	configMapName := fmt.Sprintf("%s-script", job.KubernetesJobName)

	requirements := ""
	if len(job.Dependencies) > 0 {
		for _, dep := range job.Dependencies {
			requirements += dep + "\n"
		}
	}

	configMap := &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      configMapName,
			Namespace: job.Namespace,
			Labels: map[string]string{
				JoblinManagedLabel:  JoblinManagedValue,
				JoblinNameLabel:     "script",
				JoblinInstanceLabel: job.ID,
			},
		},
		Data: map[string]string{
			"script.py":        string(job.ScriptContent),
			"requirements.txt": requirements,
		},
	}

	createdConfigMap, err := k.clientset.CoreV1().ConfigMaps(job.Namespace).Create(ctx, configMap, metav1.CreateOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to create configmap: %w", err)
	}

	return createdConfigMap, nil
}

func (k *K8sService) buildKubernetesJob(job *models.Job, configMapName string) (*batchv1.Job, error) {
	resourceRequirements := k.buildResourceRequirements(job.ResourceLimits)

	backoffLimit := int32(0)
	ttlSecondsAfterFinished := int32(int(job.TTL.Seconds()))

	scriptCommand := []string{
		"sh", "-c",
		"if [ -f /app/requirements.txt ] && [ -s /app/requirements.txt ]; then pip install -r /app/requirements.txt; fi && python /app/script.py",
	}

	k8sJob := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      job.KubernetesJobName,
			Namespace: job.Namespace,
			Labels: map[string]string{
				JoblinManagedLabel:  JoblinManagedValue,
				JoblinNameLabel:     "job",
				JoblinInstanceLabel: job.ID,
			},
		},
		Spec: batchv1.JobSpec{
			BackoffLimit:            &backoffLimit,
			TTLSecondsAfterFinished: &ttlSecondsAfterFinished,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						JoblinManagedLabel:  JoblinManagedValue,
						JoblinNameLabel:     "job",
						JoblinInstanceLabel: job.ID,
					},
				},
				Spec: corev1.PodSpec{
					RestartPolicy: corev1.RestartPolicyNever,
					Containers: []corev1.Container{
						{
							Name:    PythonScriptContainer,
							Image:   DefaultPythonImage,
							Command: scriptCommand,
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "script-volume",
									MountPath: "/app",
								},
							},
							Resources: resourceRequirements,
							Env:       buildEnvironmentVariables(job.EnvironmentVars),
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "script-volume",
							VolumeSource: corev1.VolumeSource{
								ConfigMap: &corev1.ConfigMapVolumeSource{
									LocalObjectReference: corev1.LocalObjectReference{
										Name: configMapName,
									},
									DefaultMode: int32Ptr(0755),
								},
							},
						},
					},
				},
			},
		},
	}

	if len(job.Labels) > 0 {
		for key, value := range job.Labels {
			k8sJob.Labels[key] = value
			k8sJob.Spec.Template.Labels[key] = value
		}
	}

	return k8sJob, nil
}

func (k *K8sService) buildResourceRequirements(spec models.ResourceSpec) corev1.ResourceRequirements {
	requirements := corev1.ResourceRequirements{
		Limits:   make(corev1.ResourceList),
		Requests: make(corev1.ResourceList),
	}

	if cpu, err := resource.ParseQuantity(spec.CPU); err == nil {
		requirements.Limits[corev1.ResourceCPU] = cpu
		requirements.Requests[corev1.ResourceCPU] = cpu
	}

	if memory, err := resource.ParseQuantity(spec.Memory); err == nil {
		requirements.Limits[corev1.ResourceMemory] = memory
		requirements.Requests[corev1.ResourceMemory] = memory
	}

	if storage, err := resource.ParseQuantity(spec.EphemeralStorage); err == nil {
		requirements.Limits[corev1.ResourceEphemeralStorage] = storage
		requirements.Requests[corev1.ResourceEphemeralStorage] = storage
	}

	return requirements
}

func buildEnvironmentVariables(envVars map[string]string) []corev1.EnvVar {
	if len(envVars) == 0 {
		return nil
	}

	var envList []corev1.EnvVar
	for key, value := range envVars {
		envList = append(envList, corev1.EnvVar{
			Name:  key,
			Value: value,
		})
	}

	return envList
}

func (k *K8sService) GetJobStatus(ctx context.Context, job *models.Job) (models.JobStatus, error) {
	k8sJob, err := k.clientset.BatchV1().Jobs(job.Namespace).Get(ctx, job.KubernetesJobName, metav1.GetOptions{})
	if err != nil {
		return models.StatusUnknown, fmt.Errorf("failed to get kubernetes job: %w", err)
	}

	return k.mapJobStatus(k8sJob), nil
}

func (k *K8sService) mapJobStatus(k8sJob *batchv1.Job) models.JobStatus {
	conditions := k8sJob.Status.Conditions

	for _, condition := range conditions {
		switch condition.Type {
		case batchv1.JobComplete:
			if condition.Status == corev1.ConditionTrue {
				return models.StatusCompleted
			}
		case batchv1.JobFailed:
			if condition.Status == corev1.ConditionTrue {
				return models.StatusFailed
			}
		}
	}

	if k8sJob.Status.Active > 0 {
		return models.StatusRunning
	}

	return models.StatusPending
}

func (k *K8sService) DeleteJob(ctx context.Context, job *models.Job) error {
	deletePolicy := metav1.DeletePropagationForeground

	err := k.clientset.BatchV1().Jobs(job.Namespace).Delete(ctx, job.KubernetesJobName, metav1.DeleteOptions{
		PropagationPolicy: &deletePolicy,
	})
	if err != nil {
		return fmt.Errorf("failed to delete kubernetes job: %w", err)
	}

	configMapName := fmt.Sprintf("%s-script", job.KubernetesJobName)
	err = k.clientset.CoreV1().ConfigMaps(job.Namespace).Delete(ctx, configMapName, metav1.DeleteOptions{})
	if err != nil {
		// Don't fail if configmap doesn't exist
		return nil
	}

	return nil
}

func (k *K8sService) GetJobLogs(ctx context.Context, job *models.Job, follow bool) (io.ReadCloser, error) {
	pods, err := k.getJobPods(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("failed to get job pods: %w", err)
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", job.KubernetesJobName)
	}

	pod := pods[0] // Get logs from first pod

	req := k.clientset.CoreV1().Pods(job.Namespace).GetLogs(pod.Name, &corev1.PodLogOptions{
		Container: PythonScriptContainer,
		Follow:    follow,
	})

	return req.Stream(ctx)
}

func (k *K8sService) getJobPods(ctx context.Context, job *models.Job) ([]corev1.Pod, error) {
	labelSelector := fmt.Sprintf("%s=%s,%s=%s",
		JoblinManagedLabel, JoblinManagedValue,
		JoblinInstanceLabel, job.ID)

	podList, err := k.clientset.CoreV1().Pods(job.Namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}

	return podList.Items, nil
}

func (k *K8sService) WatchJobStatus(ctx context.Context, job *models.Job, statusChan chan<- models.JobStatus) error {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			status, err := k.GetJobStatus(ctx, job)
			if err != nil {
				continue // Continue watching on error
			}

			select {
			case statusChan <- status:
			default:
				// Channel full, skip this update
			}

			if status.IsTerminal() {
				return nil
			}
		}
	}
}

func (k *K8sService) GetJobExitCode(ctx context.Context, job *models.Job) (*int, error) {
	pods, err := k.getJobPods(ctx, job)
	if err != nil {
		return nil, fmt.Errorf("failed to get job pods: %w", err)
	}

	if len(pods) == 0 {
		return nil, fmt.Errorf("no pods found for job %s", job.KubernetesJobName)
	}

	pod := pods[0]

	for _, containerStatus := range pod.Status.ContainerStatuses {
		if containerStatus.Name == PythonScriptContainer {
			if containerStatus.State.Terminated != nil {
				exitCode := int(containerStatus.State.Terminated.ExitCode)
				return &exitCode, nil
			}
		}
	}

	return nil, nil // Job not yet terminated
}

func (k *K8sService) ListJobs(ctx context.Context, namespace string) ([]*batchv1.Job, error) {
	labelSelector := fmt.Sprintf("%s=%s", JoblinManagedLabel, JoblinManagedValue)

	jobList, err := k.clientset.BatchV1().Jobs(namespace).List(ctx, metav1.ListOptions{
		LabelSelector: labelSelector,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list kubernetes jobs: %w", err)
	}

	var jobs []*batchv1.Job
	for i := range jobList.Items {
		jobs = append(jobs, &jobList.Items[i])
	}

	return jobs, nil
}

func (k *K8sService) CleanupCompletedJobs(ctx context.Context, namespace string, olderThan time.Duration) (int, error) {
	jobs, err := k.ListJobs(ctx, namespace)
	if err != nil {
		return 0, fmt.Errorf("failed to list jobs: %w", err)
	}

	var deletedCount int
	cutoff := time.Now().Add(-olderThan)

	for _, k8sJob := range jobs {
		if k8sJob.Status.CompletionTime != nil && k8sJob.Status.CompletionTime.Time.Before(cutoff) {
			deletePolicy := metav1.DeletePropagationForeground
			err := k.clientset.BatchV1().Jobs(namespace).Delete(ctx, k8sJob.Name, metav1.DeleteOptions{
				PropagationPolicy: &deletePolicy,
			})
			if err != nil {
				continue // Continue with other jobs
			}

			configMapName := fmt.Sprintf("%s-script", k8sJob.Name)
			k.clientset.CoreV1().ConfigMaps(namespace).Delete(ctx, configMapName, metav1.DeleteOptions{})

			deletedCount++
		}
	}

	return deletedCount, nil
}

func (k *K8sService) TestConnection(ctx context.Context) error {
	_, err := k.clientset.Discovery().ServerVersion()
	return err
}

func (k *K8sService) GetNamespaces(ctx context.Context) ([]string, error) {
	namespaces, err := k.clientset.CoreV1().Namespaces().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list namespaces: %w", err)
	}

	var names []string
	for _, ns := range namespaces.Items {
		names = append(names, ns.Name)
	}

	return names, nil
}

func int32Ptr(i int32) *int32 {
	return &i
}
