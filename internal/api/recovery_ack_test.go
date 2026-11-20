package api

import "testing"

func TestValidateRecoveryAcknowledgementRequest(t *testing.T) {
	valid := createPushDeploymentRequest{
		ServerIDs:                []string{"320001"},
		RecoveryAcknowledgements: map[string]string{"320001": "20261120205713-2268828"},
	}
	tests := []struct {
		name    string
		request createPushDeploymentRequest
		wantErr bool
	}{
		{name: "no acknowledgement", request: createPushDeploymentRequest{}},
		{name: "one exact target", request: valid},
		{name: "dry run", request: createPushDeploymentRequest{ServerIDs: []string{"320001"}, DryRun: true, RecoveryAcknowledgements: map[string]string{"320001": "20261120205713-2268828"}}, wantErr: true},
		{name: "all targets", request: createPushDeploymentRequest{AllProjectTargets: true, RecoveryAcknowledgements: map[string]string{"320001": "20261120205713-2268828"}}, wantErr: true},
		{name: "multiple target selectors", request: createPushDeploymentRequest{ServerIDs: []string{"320001", "320002"}, RecoveryAcknowledgements: map[string]string{"320001": "20261120205713-2268828"}}, wantErr: true},
		{name: "multiple acknowledgements", request: createPushDeploymentRequest{ServerIDs: []string{"320001"}, RecoveryAcknowledgements: map[string]string{"320001": "20261120205713-2268828", "320002": "20261120205713-2268828"}}, wantErr: true},
		{name: "target mismatch", request: createPushDeploymentRequest{ServerIDs: []string{"320001"}, RecoveryAcknowledgements: map[string]string{"320002": "20261120205713-2268828"}}, wantErr: true},
		{name: "invalid run id", request: createPushDeploymentRequest{ServerIDs: []string{"320001"}, RecoveryAcknowledgements: map[string]string{"320001": "not-a-run-id"}}, wantErr: true},
		{name: "untrimmed target", request: createPushDeploymentRequest{ServerIDs: []string{" 320001"}, RecoveryAcknowledgements: map[string]string{"320001": "20261120205713-2268828"}}, wantErr: true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			err := validateRecoveryAcknowledgementRequest(test.request)
			if (err != nil) != test.wantErr {
				t.Fatalf("error=%v wantErr=%t", err, test.wantErr)
			}
		})
	}
}
