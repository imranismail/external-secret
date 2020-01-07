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
	SecretsManagerConfig `json:"secretsManagerConfig,omitempty" yaml:"secretsManagerConfig,omitempty"`
	Data                 []Secret       `json:"data,omitempty" yaml:"data,omitempty"`
	DataFrom             []SecretSource `json:"dataFrom,omitempty" yaml:"dataFrom,omitempty"`
}

type SecretsManagerConfig struct {
	Region *string `json:"region,omitempty" yaml:"region,omitempty"`
}

type Secret struct {
	Key       *string       `json:"key,omitempty" yaml:"key,omitempty"`
	Value     *string       `json:"value,omitempty" yaml:"value,omitempty"`
	ValueFrom *SecretSource `json:"valueFrom,omitempty" yaml:"valueFrom,omitempty"`
}

type SecretSource struct {
	SecretsManagerRef `json:"secretsManagerRef,omitempty" yaml:"secretsManagerRef,omitempty"`
}

type SecretsManagerRef struct {
	Name   *string `json:"name,omitempty" yaml:"name,omitempty"`
	Key    *string `json:"key,omitempty" yaml:"key,omitempty"`
	Region *string `json:"region,omitempty" yaml:"region,omitempty"`
}

type Plugin struct {
	metav1.TypeMeta
	metav1.ObjectMeta     `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	Type                  corev1.SecretType `json:"type,omitempty" yaml:"type,omitempty"`
	Behavior              string            `json:"behavior,omitempty" yaml:"behavior,omitempty"`
	DisableNameSuffixHash bool              `json:"disableNameSuffixHash,omitempty" yaml:"disableNameSuffixHash,omitempty"`
	Spec                  Spec              `json:"spec,omitempty" yaml:"spec,omitempty"`
	cache                 map[string][]byte
}

func NewPlugin() Plugin {
	p := Plugin{}
	p.DisableNameSuffixHash = false
	p.Behavior = "create"
	p.Type = "Opaque"
	p.cache = make(map[string][]byte)

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

		if d.Key != nil {
			return fmt.Errorf("invalid input at .spec.dataFrom[%v]: `key` is not a valid option here as the content of the secret will be spreaded as key:val", i)
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
		r, err := p.GetSecretsManagerSecret(d.SecretsManagerRef)

		if err != nil {
			return nil, err
		}

		kv := make(map[string][]byte)
		err = json.Unmarshal(r, &kv)

		if err != nil {
			return nil, err
		}

		for k, v := range kv {
			s.Data[k] = v
		}
	}

	for _, d := range p.Spec.Data {
		if d.Value != nil {
			s.Data[*d.Key] = []byte(*d.Value)
		} else if d.ValueFrom != nil {
			r, err := p.GetSecretsManagerSecret(d.ValueFrom.SecretsManagerRef)

			if err != nil {
				return nil, err
			}

			if d.ValueFrom.Key == nil {
				s.Data[*d.Key] = r
			} else {
				kv := make(map[string][]byte)
				err = json.Unmarshal(r, &kv)

				if err != nil {
					return nil, err
				}

				if v, ok := kv[*d.ValueFrom.Key]; ok {
					s.Data[*d.Key] = v
				} else {
					return nil, fmt.Errorf("invalid secret: missing key `%v` in secret `%v`", *d.ValueFrom.Key, *d.ValueFrom.Name)
				}
			}
		}
	}

	return &s, nil
}

func (p *Plugin) GetSecretsManagerSvc(r *string) (*secretsmanager.SecretsManager, error) {
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

func (p *Plugin) GetSecretsManagerSecret(s SecretsManagerRef) ([]byte, error) {
	n := s.Name
	r := s.Region

	if r == nil {
		r = p.Spec.SecretsManagerConfig.Region
	}

	ck := *n

	if r != nil {
		ck = fmt.Sprintf("%s:%s", *r, ck)
	}

	if val, ok := p.cache[ck]; ok {
		return val, nil
	}

	svc, err := p.GetSecretsManagerSvc(r)

	if err != nil {
		return nil, err
	}

	res, err := svc.GetSecretValue(&secretsmanager.GetSecretValueInput{
		SecretId: n,
	})

	if err != nil {
		return nil, err
	}

	if res.SecretString != nil {
		p.cache[ck] = []byte(*res.SecretString)
	} else if res.SecretBinary != nil {
		p.cache[ck] = res.SecretBinary
	} else {
		return nil, fmt.Errorf("invalid secret: no string or binary found, make sure the content of secret `%v` is not empty", n)
	}

	return p.cache[ck], nil
}

func (s *Secret) Validate() error {
	if s.Value != nil && s.ValueFrom != nil {
		return fmt.Errorf("only one of `value` or `valueFrom` is permitted")
	}

	if s.Value == nil && s.ValueFrom == nil {
		return fmt.Errorf("one of `value` or `valueFrom` is required")
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
			return fmt.Errorf("only one of the supported secret provider is permitted")
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
