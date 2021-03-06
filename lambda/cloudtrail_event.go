package lambda

// CloudTrail represents a CloudTrail event that's autogenerated
// from https://github.com/ChimeraCoder/gojson
type CloudTrail struct {
	Account string `json:"account"`
	Detail  struct {
		AdditionalEventData struct {
			AuthenticationMethod string  `json:"AuthenticationMethod"`
			CipherSuite          string  `json:"CipherSuite"`
			SignatureVersion     string  `json:"SignatureVersion"`
			BytesTransferredIn   float64 `json:"bytesTransferredIn"`
			BytesTransferredOut  float64 `json:"bytesTransferredOut"`
			XAmzID2go            string  `json:"x-amz-id-2"`
		} `json:"additionalEventData"`
		AwsRegion          string `json:"awsRegion"`
		EventCategory      string `json:"eventCategory"`
		EventID            string `json:"eventID"`
		EventName          string `json:"eventName"`
		EventSource        string `json:"eventSource"`
		EventTime          string `json:"eventTime"`
		EventType          string `json:"eventType"`
		EventVersion       string `json:"eventVersion"`
		ManagementEvent    bool   `json:"managementEvent"`
		ReadOnly           bool   `json:"readOnly"`
		RecipientAccountID string `json:"recipientAccountId"`
		RequestID          string `json:"requestID"`
		RequestParameters  struct {
			Host       string `json:"Host"`
			BucketName string `json:"bucketName"`
			Key        string `json:"key"`
		} `json:"requestParameters"`
		Resources []struct {
			Arn       string `json:"ARN"`
			AccountID string `json:"accountId"`
			Type      string `json:"type"`
		} `json:"resources"`
		ResponseElements interface{} `json:"responseElements"`
		SourceIPAddress  string      `json:"sourceIPAddress"`
		UserAgent        string      `json:"userAgent"`
		UserIdentity     struct {
			AccessKeyID string `json:"accessKeyId"`
			AccountID   string `json:"accountId"`
			Arn         string `json:"arn"`
			PrincipalID string `json:"principalId"`
			Type        string `json:"type"`
		} `json:"userIdentity"`
	} `json:"detail"`
	DetailType string        `json:"detail-type"`
	ID         string        `json:"id"`
	Region     string        `json:"region"`
	Resources  []interface{} `json:"resources"`
	Source     string        `json:"source"`
	Time       string        `json:"time"`
	Version    string        `json:"version"`
}
