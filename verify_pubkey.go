package paymail

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"

	"github.com/go-resty/resty/v2"
)

/*
Default:

{
  "handle":"somepaymailhandle@domain.tld",
  "match": true,
  "pubkey":"<consulted pubkey>"
}
*/

// Verification is the result returned from the VerifyPubKey() request
type Verification struct {
	StandardResponse
	BsvAlias string `json:"bsvalias"` // Version of the bsvalias
	Handle   string `json:"handle"`   // The <alias>@<domain>.<tld>
	Match    bool   `json:"match"`    // If the match was successful or not
	PubKey   string `json:"pubkey"`   // The related PubKey
}

// VerifyPubKey will try to match a handle and pubkey
//
// Specs: https://bsvalias.org/05-verify-public-key-owner.html
func (c *Client) VerifyPubKey(verifyURL, alias, domain, pubKey string) (response *Verification, err error) {

	// Require a valid url
	if len(verifyURL) == 0 || !strings.Contains(verifyURL, "https://") {
		err = fmt.Errorf("invalid url: %s", verifyURL)
		return
	}

	// Basic requirements for resolution request
	if len(alias) == 0 {
		err = fmt.Errorf("missing alias")
		return
	} else if len(domain) == 0 {
		err = fmt.Errorf("missing domain")
		return
	} else if len(pubKey) == 0 {
		err = fmt.Errorf("missing pubKey")
		return
	}

	// Set the base url and path, assuming the url is from the prior GetCapabilities() request
	// https://<host-discovery-target>/verifypubkey/{alias}@{domain.tld}/{pubkey}
	reqURL := strings.Replace(strings.Replace(strings.Replace(verifyURL, "{pubkey}", pubKey, -1), "{alias}", alias, -1), "{domain.tld}", domain, -1)

	// Set the user agent
	req := c.Resty.R().SetHeader("User-Agent", c.Options.UserAgent)

	// Enable tracing
	if c.Options.RequestTracing {
		req.EnableTrace()
	}

	// Fire the request
	var resp *resty.Response
	if resp, err = req.Get(reqURL); err != nil {
		return
	}

	// New struct
	response = new(Verification)

	// Tracing enabled?
	if c.Options.RequestTracing {
		response.Tracing = resp.Request.TraceInfo()
	}

	// Test the status code (200 or 304 is valid)
	response.StatusCode = resp.StatusCode()
	if response.StatusCode != http.StatusOK && response.StatusCode != http.StatusNotModified {
		je := &JSONError{}
		if err = json.Unmarshal(resp.Body(), je); err != nil {
			return
		}
		err = fmt.Errorf("bad response from paymail provider: code %d, message: %s", response.StatusCode, je.Message)
		return
	}

	// Decode the body of the response
	if err = json.Unmarshal(resp.Body(), &response); err != nil {
		return
	}

	// Invalid version?
	if len(response.BsvAlias) == 0 {
		err = fmt.Errorf("missing bsvalias version")
		return
	}

	// Check basic requirements (alias@domain.tld)
	if response.Handle != alias+"@"+domain {
		err = fmt.Errorf("verify response handle %s does not match paymail address: %s", response.Handle, alias+"@"+domain)
		return
	}

	// Check the PubKey length
	if len(response.PubKey) == 0 {
		err = fmt.Errorf("pki response is missing a PubKey value")
	} else if len(response.PubKey) != PubKeyLength {
		err = fmt.Errorf("returned pubkey is not the required length of %d, got: %d", PubKeyLength, len(response.PubKey))
	}

	return
}