package provider

import (
	"context"
	"fmt"
	"log"
	"regexp"
	"testing"

	elastic7 "github.com/olivere/elastic/v7"
	elastic6 "gopkg.in/olivere/elastic.v6"

	"github.com/hashicorp/terraform-plugin-sdk/v2/helper/resource"
	"github.com/hashicorp/terraform-plugin-sdk/v2/terraform"
)

func TestAccOpensearchDashboardObject(t *testing.T) {
	provider := Provider()
	diags := provider.Configure(context.Background(), &terraform.ResourceConfig{})
	if diags.HasError() {
		t.Skipf("err: %#v", diags)
	}

	visualizationConfig := testAccOpensearch7DashboardVisualization
	indexPatternConfig := testAccOpensearch7DashboardIndexPattern

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testCheckOpensearchDashboardObjectDestroy,
		Steps: []resource.TestStep{
			{
				Config: visualizationConfig,
				Check: resource.ComposeTestCheckFunc(
					testCheckOpensearchDashboardObjectExists("opensearch_dashboard_object.test_visualization", "response-time-percentile", ""),
				),
			},
			{
				Config: indexPatternConfig,
				Check: resource.ComposeTestCheckFunc(
					testCheckOpensearchDashboardObjectExists("opensearch_dashboard_object.test_pattern", "index-pattern:cloudwatch", ""),
				),
			},
		},
	})
}

func TestAccOpensearchDashboardObjectWithTenant(t *testing.T) {
	provider := Provider()
	diags := provider.Configure(context.Background(), &terraform.ResourceConfig{})
	if diags.HasError() {
		t.Skipf("err: %#v", diags)
	}

	visualizationConfig := testAccOpensearch7DashboardVisualizationWithTenant
	indexPatternConfig := testAccOpensearch7DashboardIndexPatternWithTenant

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testCheckOpensearchDashboardObjectDestroyWithTenant,
		Steps: []resource.TestStep{
			{
				Config: visualizationConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("opensearch_dashboard_tenant.tenant_test", "tenant_name", "tenant_test"),
					testCheckOpensearchDashboardObjectExists("opensearch_dashboard_object.test_visualization", "response-time-percentile", "tenant_test"),
				),
			},
			{
				Config: indexPatternConfig,
				Check: resource.ComposeTestCheckFunc(
					resource.TestCheckResourceAttr("opensearch_dashboard_tenant.tenant_test", "tenant_name", "tenant_test"),
					testCheckOpensearchDashboardObjectExists("opensearch_dashboard_object.test_pattern", "index-pattern:cloudwatch", "tenant_test"),
				),
			},
		},
	})
}

func TestAccOpensearchDashboardObject_ProviderFormatInvalid(t *testing.T) {
	provider := Provider()
	diags := provider.Configure(context.Background(), &terraform.ResourceConfig{})
	if diags.HasError() {
		t.Skipf("err: %#v", diags)
	}

	resource.Test(t, resource.TestCase{
		Providers:    testAccProviders,
		CheckDestroy: testCheckOpensearchDashboardObjectDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccOpensearchFormatInvalid,
				ExpectError: regexp.MustCompile("must be an array of objects"),
			},
		},
	})
}

func TestAccOpensearchDashboardObject_Rejected(t *testing.T) {
	provider := Provider()
	diags := provider.Configure(context.Background(), &terraform.ResourceConfig{})
	if diags.HasError() {
		t.Skipf("err: %#v", diags)
	}
	var allowed bool = false

	resource.Test(t, resource.TestCase{
		PreCheck: func() {
			testAccPreCheck(t)
			if !allowed {
				t.Skip("Only >= OS 2.0.0 has index type restrictions")
			}
		},
		Providers:    testAccProviders,
		CheckDestroy: testCheckOpensearchDashboardObjectDestroy,
		Steps: []resource.TestStep{
			{
				Config:      testAccOpensearchDashboardIndexPattern,
				ExpectError: regexp.MustCompile("Error 400"),
			},
		},
	})
}

func testCheckOpensearchDashboardObjectExists(name string, id string, tenantName string) resource.TestCheckFunc {
	return func(s *terraform.State) error {
		rs, ok := s.RootModule().Resources[name]
		if !ok {
			return fmt.Errorf("Not found: %s", name)
		}
		if rs.Primary.ID == "" {
			return fmt.Errorf("No Dashboard object ID is set")
		}

		meta := testAccProvider.Meta()

		var err error
		osClient, err := getClient(meta.(*ProviderConf))
		if err != nil {
			return err
		}
		_, err = osClient.Get().
			Index(".kibana").
			Id(id).
			Header(SECURITY_TENANT_HEADER, tenantName).
			Do(context.TODO())

		if err != nil {
			log.Printf("[INFO] testCheckOpensearchDashboardObjectExists: %+v", err)
			return err
		}

		return nil
	}
}

func testCheckOpensearchDashboardObjectDestroy(s *terraform.State) error {
	for _, rs := range s.RootModule().Resources {
		if rs.Type != "opensearch_dashboard_object" {
			continue
		}

		meta := testAccProvider.Meta()
		tenantName := rs.Primary.Attributes["tenant_name"]

		osClient, err := getClient(meta.(*ProviderConf))
		if err != nil {
			return err
		}
		_, err = osClient.Get().
			Index(".kibana").
			Id("response-time-percentile").
			Header(SECURITY_TENANT_HEADER, tenantName).
			Do(context.TODO())

		if err != nil {
			if elastic7.IsNotFound(err) || elastic6.IsNotFound(err) {
				return nil // should be not found error
			}

			if tenantName != "global_tenant" && (elastic7.IsForbidden(err) || elastic6.IsForbidden(err)) {
				// when tenant has been destroyed this is the expected error
				return nil
			}

			// Fail on any other error
			return fmt.Errorf("Unexpected error %s", err)
		}

		return fmt.Errorf("Dashboard object %q still exists", rs.Primary.ID)
	}

	return nil
}

func testCheckOpensearchDashboardObjectDestroyWithTenant(s *terraform.State) error {
	if err := testCheckOpensearchDashboardObjectDestroy(s); err != nil {
		return err
	}
	if err := testAccCheckOpensearchDashboardTenantDestroy(s); err != nil {
		return err
	}
	return nil
}

var testAccOpensearch7DashboardVisualization = `
resource "opensearch_dashboard_object" "test_visualization" {
  body = <<EOF
[
  {
    "_id": "response-time-percentile",
    "_source": {
      "visualization": {
	      "title": "Total response time percentiles",
	      "visState": "{\"title\":\"Total response time percentiles\",\"type\":\"line\",\"params\":{\"addTooltip\":true,\"addLegend\":true,\"legendPosition\":\"right\",\"showCircles\":true,\"interpolate\":\"linear\",\"scale\":\"linear\",\"drawLinesBetweenPoints\":true,\"radiusRatio\":9,\"times\":[],\"addTimeMarker\":false,\"defaultYExtents\":false,\"setYExtents\":false},\"aggs\":[{\"id\":\"1\",\"enabled\":true,\"type\":\"percentiles\",\"schema\":\"metric\",\"params\":{\"field\":\"app.total_time\",\"percents\":[50,90,95]}},{\"id\":\"2\",\"enabled\":true,\"type\":\"date_histogram\",\"schema\":\"segment\",\"params\":{\"field\":\"@timestamp\",\"interval\":\"auto\",\"customInterval\":\"2h\",\"min_doc_count\":1,\"extended_bounds\":{}}},{\"id\":\"3\",\"enabled\":true,\"type\":\"terms\",\"schema\":\"group\",\"params\":{\"field\":\"system.syslog.program\",\"size\":5,\"order\":\"desc\",\"orderBy\":\"_term\"}}],\"listeners\":{}}",
	      "uiStateJSON": "{}",
	      "description": "",
	      "version": 1,
	      "kibanaSavedObjectMeta": {
	        "searchSourceJSON": "{\"index\":\"filebeat-*\",\"query\":{\"query_string\":{\"query\":\"*\",\"analyze_wildcard\":true}},\"filter\":[]}"
	      }
	    },
      "type": "visualization"
    }
  }
]
EOF
}
`

var testAccOpensearch7DashboardVisualizationWithTenant = `
resource "opensearch_dashboard_tenant" "tenant_test" {
  tenant_name = "tenant_test"
  description = "tenant_test"
}

resource "opensearch_dashboard_object" "test_visualization" {
  depends_on = [
    opensearch_dashboard_tenant.tenant_test
  ]
  tenant_name = "tenant_test"
  body        = <<EOF
[
  {
    "_id": "response-time-percentile",
    "_source": {
      "visualization": {
	      "title": "Total response time percentiles",
	      "visState": "{\"title\":\"Total response time percentiles\",\"type\":\"line\",\"params\":{\"addTooltip\":true,\"addLegend\":true,\"legendPosition\":\"right\",\"showCircles\":true,\"interpolate\":\"linear\",\"scale\":\"linear\",\"drawLinesBetweenPoints\":true,\"radiusRatio\":9,\"times\":[],\"addTimeMarker\":false,\"defaultYExtents\":false,\"setYExtents\":false},\"aggs\":[{\"id\":\"1\",\"enabled\":true,\"type\":\"percentiles\",\"schema\":\"metric\",\"params\":{\"field\":\"app.total_time\",\"percents\":[50,90,95]}},{\"id\":\"2\",\"enabled\":true,\"type\":\"date_histogram\",\"schema\":\"segment\",\"params\":{\"field\":\"@timestamp\",\"interval\":\"auto\",\"customInterval\":\"2h\",\"min_doc_count\":1,\"extended_bounds\":{}}},{\"id\":\"3\",\"enabled\":true,\"type\":\"terms\",\"schema\":\"group\",\"params\":{\"field\":\"system.syslog.program\",\"size\":5,\"order\":\"desc\",\"orderBy\":\"_term\"}}],\"listeners\":{}}",
	      "uiStateJSON": "{}",
	      "description": "",
	      "version": 1,
	      "kibanaSavedObjectMeta": {
	        "searchSourceJSON": "{\"index\":\"filebeat-*\",\"query\":{\"query_string\":{\"query\":\"*\",\"analyze_wildcard\":true}},\"filter\":[]}"
	      }
	    },
      "type": "visualization"
    }
  }
]
EOF
}
`

var testAccOpensearchDashboardIndexPattern = `
resource "opensearch_dashboard_object" "test_pattern" {
  body = <<EOF
[
  {
		"_id": "index-pattern:cloudwatch",
		"_type": "index-pattern",
		"_source": {
			"title": "cloudwatch-*",
			"timeFieldName": "timestamp"
		}
	}
]
EOF
}
`

var testAccOpensearch7DashboardIndexPattern = `
resource "opensearch_dashboard_object" "test_pattern" {
  index = ".kibana"
  body  = <<EOF
[
  {
		"_id": "index-pattern:cloudwatch",
		"_source": {
			"type": "index-pattern",
			"index-pattern": {
				"title": "cloudwatch-*",
				"timeFieldName": "@timestamp"
			}
		}
	}
]
EOF
}
`

var testAccOpensearch7DashboardIndexPatternWithTenant = `
resource "opensearch_dashboard_tenant" "tenant_test" {
  tenant_name = "tenant_test"
  description = "tenant_test"
}

resource "opensearch_dashboard_object" "test_pattern" {
  depends_on = [
    opensearch_dashboard_tenant.tenant_test
  ]
  tenant_name = "tenant_test"
  body        = <<EOF
[
  {
		"_id": "index-pattern:cloudwatch",
		"_source": {
			"type": "index-pattern",
			"index-pattern": {
				"title": "cloudwatch-*",
				"timeFieldName": "@timestamp"
			}
		}
	}
]
EOF
}
`

var testAccOpensearchFormatInvalid = `
resource "opensearch_dashboard_object" "test_invalid" {
  body = <<EOF
{
  "test": "yes"
}
EOF
}
`
