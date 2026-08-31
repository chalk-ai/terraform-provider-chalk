package provider

import (
	"fmt"

	serverv1 "github.com/chalk-ai/chalk-go/gen/chalk/server/v1"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"google.golang.org/protobuf/encoding/protowire"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// certificateIssuerRefFieldNumber is the field number of certificate_issuer_ref on
// EnvoyGatewayProviderConfig in chalk/server/v1/builder.proto. The field landed in
// chalk-private before it was published in chalk-go, so encode it as a protobuf unknown
// field to keep the provider compatible with both the old and new generated clients.
const certificateIssuerRefFieldNumber protowire.Number = 14

func setCertificateIssuerRefOnProto(config *serverv1.EnvoyGatewayProviderConfig, ref *CertificateIssuerRefModel) {
	if ref == nil {
		return
	}
	message := config.ProtoReflect()
	if field := message.Descriptor().Fields().ByNumber(certificateIssuerRefFieldNumber); field != nil {
		issuer := message.Mutable(field).Message()
		issuer.Set(issuer.Descriptor().Fields().ByNumber(1), protoreflect.ValueOfString(ref.Name.ValueString()))
		issuer.Set(issuer.Descriptor().Fields().ByNumber(2), protoreflect.ValueOfString(ref.Kind.ValueString()))
		issuer.Set(issuer.Descriptor().Fields().ByNumber(3), protoreflect.ValueOfString(ref.Group.ValueString()))
		return
	}

	payload := make([]byte, 0)
	payload = appendProtoString(payload, 1, ref.Name.ValueString())
	payload = appendProtoString(payload, 2, ref.Kind.ValueString())
	payload = appendProtoString(payload, 3, ref.Group.ValueString())

	unknown := message.GetUnknown()
	unknown = protowire.AppendTag(unknown, certificateIssuerRefFieldNumber, protowire.BytesType)
	unknown = protowire.AppendBytes(unknown, payload)
	message.SetUnknown(unknown)
}

func certificateIssuerRefFromProto(config *serverv1.EnvoyGatewayProviderConfig) (*CertificateIssuerRefModel, error) {
	message := config.ProtoReflect()
	if field := message.Descriptor().Fields().ByNumber(certificateIssuerRefFieldNumber); field != nil && message.Has(field) {
		issuer := message.Get(field).Message()
		return newCertificateIssuerRefModel(
			issuer.Get(issuer.Descriptor().Fields().ByNumber(1)).String(),
			issuer.Get(issuer.Descriptor().Fields().ByNumber(2)).String(),
			issuer.Get(issuer.Descriptor().Fields().ByNumber(3)).String(),
		)
	}

	unknown := message.GetUnknown()
	for len(unknown) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(unknown)
		if tagLength < 0 {
			return nil, protowire.ParseError(tagLength)
		}
		unknown = unknown[tagLength:]

		if number == certificateIssuerRefFieldNumber && wireType == protowire.BytesType {
			payload, valueLength := protowire.ConsumeBytes(unknown)
			if valueLength < 0 {
				return nil, protowire.ParseError(valueLength)
			}
			return parseCertificateIssuerRef(payload)
		}

		valueLength := protowire.ConsumeFieldValue(number, wireType, unknown)
		if valueLength < 0 {
			return nil, protowire.ParseError(valueLength)
		}
		unknown = unknown[valueLength:]
	}

	return nil, nil
}

func parseCertificateIssuerRef(payload []byte) (*CertificateIssuerRefModel, error) {
	var name, kind, group string
	for len(payload) > 0 {
		number, wireType, tagLength := protowire.ConsumeTag(payload)
		if tagLength < 0 {
			return nil, protowire.ParseError(tagLength)
		}
		payload = payload[tagLength:]

		if wireType == protowire.BytesType && (number == 1 || number == 2 || number == 3) {
			value, valueLength := protowire.ConsumeString(payload)
			if valueLength < 0 {
				return nil, protowire.ParseError(valueLength)
			}
			switch number {
			case 1:
				name = value
			case 2:
				kind = value
			case 3:
				group = value
			}
			payload = payload[valueLength:]
			continue
		}

		valueLength := protowire.ConsumeFieldValue(number, wireType, payload)
		if valueLength < 0 {
			return nil, protowire.ParseError(valueLength)
		}
		payload = payload[valueLength:]
	}

	return newCertificateIssuerRefModel(name, kind, group)
}

func newCertificateIssuerRefModel(name, kind, group string) (*CertificateIssuerRefModel, error) {
	if name == "" || kind == "" || group == "" {
		return nil, fmt.Errorf("name, kind, and group are required")
	}

	return &CertificateIssuerRefModel{
		Name:  types.StringValue(name),
		Kind:  types.StringValue(kind),
		Group: types.StringValue(group),
	}, nil
}

func appendProtoString(buffer []byte, number protowire.Number, value string) []byte {
	buffer = protowire.AppendTag(buffer, number, protowire.BytesType)
	return protowire.AppendString(buffer, value)
}
