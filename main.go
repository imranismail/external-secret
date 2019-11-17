package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"reflect"
	"strconv"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"sigs.k8s.io/yaml"
)

type Spec struct {
	SecretManagerConfig `json:"secretManagerConfig,omitempty" yaml:"secretManagerConfig,omitempty"`
	Data                []Secret           `json:"data,omitempty" yaml:"data,omitempty"`
	DataFrom            []SecretFromSource `json:"dataFrom,omitempty" yaml:"dataFrom,omitempty"`
}

type SecretManagerConfig struct {
	Region *string `json:"region,omitempty" yaml:"region,omitempty"`
}

type Secret struct {
	Key       *string       `json:"key,omitempty" yaml:"key,omitempty"`
	Value     *string       `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *SecretSource `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}

type SecretFromSource struct {
	SecretManagerRef `json:"secretManagerRef,omitempty" yaml:"secretManagerRef,omitempty"`
}

type SecretSource struct {
	SecretManagerKeyRef `json:"secretManagerKeyRef,omitempty" yaml:"secretManagerKeyRef,omitempty"`
}

type SecretManagerKeyRef struct {
	Name   *string `json:"name,omitempty" yaml:"name,omitempty"`
	Key    *string `json:"key,omitempty" yaml:"key,omitempty"`
	Region *string `json:"region,omitempty" yaml:"region,omitempty"`
}

type SecretManagerRef struct {
	Name   *string `json:"name,omitempty" yaml:"name,omitempty"`
	Region *string `json:"region,omitempty" yaml:"region,omitempty"`
}

type Plugin struct {
	metav1.TypeMeta
	metav1.ObjectMeta     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Type                  corev1.SecretType `json:"type,omitempty" yaml:"type,omitempty"`
	Behavior              string            `json:"behavior,omitempty" yaml:"behavior,omitempty"`
	DisableNameSuffixHash bool              `json:"disableNameSuffixHash,omitempty" yaml:"disableNameSuffixHash,omitempty"`
	Spec                  Spec              `json:"spec,omitempty" yaml:"spec,omitempty"`
	cache                 map[string]string
}

type AWSSecretRef interface {
	GetName() *string
	GetRegion() *string
}

func (s SecretManagerKeyRef) GetName() *string {
	return s.Name
}

func (s SecretManagerKeyRef) GetRegion() *string {
	return s.Region
}

func (s SecretManagerRef) GetName() *string {
	return s.Name
}

func (s SecretManagerRef) GetRegion() *string {
	return s.Region
}

func NewPlugin() Plugin {
	p := Plugin{}
	p.DisableNameSuffixHash = false
	p.Behavior = "create"
	p.Type = "Opaque"
	p.cache = make(map[string]string)

	return p
}

func (p *Plugin) Read(fileName string) error {
	b, err := ioutil.ReadFile(os.Args[1])

	if err != nil {
		return err
	}

	err = yaml.Unmarshal(b, p)

	if err != nil {
		return err
	}

	return p.Validate()
}

func (p *Plugin) Validate() error {
	for i, d := range p.Spec.DataFrom {
		err := d.Validate()

		if err != nil {
			return fmt.Errorf("invalid input at .spec.dataFrom[%v]: %s", i, err)
		}
	}

	for i, d := range p.Spec.Data {
		err := d.Validate()

		if err != nil {
			return fmt.Errorf("invalid input at .spec.data[%v]: %s", i, err)
		}
	}

	return nil
}

func (p *Plugin) GenerateSecret() (*corev1.Secret, error) {
	s := corev1.Secret{}
	s.SetGroupVersionKind(schema.FromAPIVersionAndKind("v1", "Secret"))
	s.SetName(p.GetName())
	s.SetNamespace(p.GetNamespace())
	s.SetLabels(p.GetLabels())
	s.Type = p.Type
	s.Data = make(map[string][]byte)

	a := make(map[string]string)
	d := map[string]string{
		"kustomize.config.k8s.io/needs-hash": strconv.FormatBool(!p.DisableNameSuffixHash),
		"kustomize.config.k8s.io/behavior":   p.Behavior,
	}

	for k, v := range p.GetAnnotations() {
		a[k] = v
	}

	for k, v := range d {
		a[k] = v
	}

	s.SetAnnotations(a)

	for _, d := range p.Spec.DataFrom {
		r, err := p.GetSecretManagerSecret(d.SecretManagerRef)

		if err != nil {
			return nil, err
		}

		kv := make(map[string]string)
		err = json.Unmarshal([]byte(r), &kv)

		if err != nil {
			return nil, err
		}

		for k, v := range kv {
			s.Data[k] = []byte(v)
		}
	}

	for _, d := range p.Spec.Data {
		if d.Value != nil {
			s.Data[*d.Key] = []byte(*d.Value)
		}

		if d.ValueFrom != nil {
			r, err := p.GetSecretManagerSecret(d.ValueFrom.SecretManagerKeyRef)

			if err != nil {
				return nil, err
			}

			if d.ValueFrom.SecretManagerKeyRef.Key == nil {
				s.Data[*d.Key] = []byte(r)
			} else {
				kv := make(map[string]string)
				err = json.Unmarshal([]byte(r), &kv)

				if err != nil {
					return nil, err
				}

				if v, ok := kv[*d.ValueFrom.SecretManagerKeyRef.Key]; ok {
					s.Data[*d.Key] = []byte(v)
				} else {
					return nil, fmt.Errorf("Missing key %v in secret %v", *d.ValueFrom.SecretManagerKeyRef.Key, *d.ValueFrom.GetName())
				}
			}
		}
	}

	return &s, nil
}

func (p *Plugin) GetSecretManagerSvc(r *string) (*secretsmanager.SecretsManager, error) {
	sess, err := session.NewSession(
		&aws.Config{
			Region: r,
		},
	)

	if err != nil {
		return nil, err
	}

	svc := secretsmanager.New(sess)

	return svc, nil
}

func (p *Plugin) GetSecretManagerSecret(s AWSSecretRef) (string, error) {
	n := s.GetName()
	r := s.GetRegion()

	if r == nil {
		r = p.Spec.SecretManagerConfig.Region
	}

	ck := *n

	if r != nil {
		ck = fmt.Sprintf("%s:%s", *r, ck)
	}

	if val, ok := p.cache[ck]; ok {
		return val, nil
	}

	svc, err := p.GetSecretManagerSvc(r)

	if err != nil {
		return "", err
	}

	res, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: n,
	})

	if err != nil {
		return "", err
	}

	p.cache[ck] = *res.SecretString

	return p.cache[ck], nil
}

func (s *Secret) Validate() error {
	if s.Value != nil && s.ValueFrom != nil {
		return fmt.Errorf("you may only specify one of `value` or `valueFrom`")
	}

	if s.ValueFrom != nil {
		return s.ValueFrom.Validate()
	}

	return nil
}

func (s *SecretSource) Validate() error {
	v := reflect.ValueOf(*s)
	n := v.NumField()

	for i := 0; i < n; i++ {
		cv := v.Field(i)

		if i > 0 && !cv.IsNil() {
			return fmt.Errorf("you may only specify one of `secretManagerKeyRef` or `runtimeConfiguratorKeyRef`")
		}
	}

	return nil
}

func (s *SecretFromSource) Validate() error {
	v := reflect.ValueOf(*s)
	n := v.NumField()

	for i := 0; i < n; i++ {
		cv := v.Field(i)

		if i > 0 && !cv.IsNil() {
			return fmt.Errorf("you may only specify one of `secretManagerRef` or `runtimeConfiguratorRef`")
		}
	}

	return nil
}

func main() {
	p := NewPlugin()
	err := p.Read(os.Args[1])

	if err != nil {
		log.Fatal(err)
	}

	s, err := p.GenerateSecret()

	if err != nil {
		log.Fatal(err)
	}

	b, err := yaml.Marshal(s)

	if err != nil {
		log.Fatal(err)
	}

	fmt.Print(string(b))
}
