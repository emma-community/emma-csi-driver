# Emma API Discovery Results

This document contains real data discovered from the Emma API to help with CSI driver configuration.

## API Status
✅ **Authentication**: Working correctly
✅ **Data Centers**: 256 available across multiple providers
✅ **VMs**: API accessible (0 VMs in test account)
✅ **Volumes**: API accessible (1 existing volume found)

## Available Data Centers

Emma supports **256 data centers** across 8 cloud providers:

### Providers Summary
- **Azure**: 36 regions (australiaeast, eastus, westeurope, etc.)
- **AWS**: 29 regions (us-east-1, eu-west-1, ap-southeast-1, etc.)
- **GCP**: 133 zones across multiple regions
- **OVHcloud**: 25 regions (EU, North America, Asia)
- **IONOS**: 9 regions (Europe and North America)
- **DigitalOcean**: 12 regions
- **Gcore**: 21 regions (Global coverage)
- **VMware**: 1 datacenter

### Popular Data Centers for Testing

#### AWS Regions
```yaml
- aws-us-east-1      # Virginia
- aws-us-west-2      # Oregon
- aws-eu-west-1      # Dublin
- aws-eu-central-1   # Frankfurt
- aws-ap-southeast-1 # Singapore
```

#### Azure Regions
```yaml
- azure-eastus       # Virginia
- azure-westus2      # Washington
- azure-westeurope   # Amsterdam
- azure-northeurope  # Dublin
- azure-southeastasia # Singapore
```

#### GCP Zones
```yaml
- gcp-us-central1-a  # Iowa
- gcp-us-east1-b     # South Carolina
- gcp-europe-west1-b # Belgium
- gcp-asia-east1-a   # Taiwan
```

## Volume Configuration

### Existing Volume Example
```yaml
ID: 93728
Name: volume-24s5ru
Size: 32GB
Status: ACTIVE
Attached: VM 93727
DataCenter: (varies by VM location)
```

### Volume Types
✅ **Tested and Working**:
- `ssd` - SSD-backed storage (recommended for most workloads)

**Other possible types** (not tested):
- `hdd` - HDD-backed storage (cost-effective for large data)
- `premium` - High-performance SSD (for I/O intensive workloads)

### Volume Sizes
⚠️ **Important**: Volume sizes must match Emma's available configurations.
- **Tested working size**: 32GB
- The API will reject sizes that don't match available options
- Error message: "The volume size must correspond to one of the available options"

**Note**: Volume configurations endpoint returned 400 Bad Request, suggesting it may require specific parameters or different authentication scope. Without this endpoint, we cannot programmatically discover valid sizes.

## StorageClass Examples

### AWS Example
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: emma-aws-us-east
provisioner: csi.emma.ms
parameters:
  dataCenterId: "aws-us-east-1"
  type: "ssd"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```

### Azure Example
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: emma-azure-westeurope
provisioner: csi.emma.ms
parameters:
  dataCenterId: "azure-westeurope"
  type: "premium"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```

### GCP Example
```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: emma-gcp-us-central
provisioner: csi.emma.ms
parameters:
  dataCenterId: "gcp-us-central1-a"
  type: "ssd"
allowVolumeExpansion: true
volumeBindingMode: WaitForFirstConsumer
```

## Recommendations

### 1. Default Data Center Selection
Since there are 256 data centers, consider:
- Making `dataCenterId` **required** in StorageClass parameters
- Providing clear documentation on how to list available data centers
- Adding validation in the controller to fail fast with helpful error messages

### 2. Data Center Discovery
Add this to controller startup (already implemented in task 3):
```go
datacenters, err := emmaClient.GetDataCenters(ctx)
if err == nil {
    logger.Info("Available datacenters", map[string]interface{}{
        "count": len(datacenters),
    })
}
```

### 3. Volume Type Validation
Since we couldn't fetch volume configurations, consider:
- Accepting common types: `ssd`, `hdd`, `premium`
- Letting the Emma API validate the type during volume creation
- Providing clear error messages when invalid types are used

### 4. Testing Strategy
For integration tests, use:
- **Test DataCenter**: `aws-us-east-1` (most common)
- **Test Volume Type**: `ssd` (widely supported)
- **Test Size**: 10GB minimum (check Emma API limits)

## API Limitations Discovered

1. **Volume Configurations Endpoint**: Returns 400 Bad Request
   - May require specific query parameters
   - May need different API scope/permissions
   - Fallback: Use common volume types and let API validate

2. **No VMs in Test Account**: 
   - Attach/detach operations cannot be fully tested
   - Consider documenting VM requirements for users

## Volume Creation Test Results

✅ **Successfully tested volume lifecycle**:
1. Create volume: `POST /v1/volumes` - Working
2. Get volume: `GET /v1/volumes/{id}` - Working
3. Wait for status: Polling until AVAILABLE - Working
4. Delete volume: `DELETE /v1/volumes/{id}` - Working

**Test Details**:
- DataCenter: `aws-us-east-1`
- Volume Type: `ssd`
- Volume Size: `32GB`
- Status Flow: `DRAFT` → `AVAILABLE` (within 2 minutes)
- Volume ID: Auto-generated by Emma API
- Volume Name: Auto-generated (format: `volume-XXXXXX`)

## API Field Name Corrections

The Emma API uses different field names than initially expected:
- ✅ `volumeGb` (not `sizeGb`)
- ✅ `volumeType` (not `type`)
- ✅ `dataCenterId` (correct)
- ✅ `name` (correct, but API may override with auto-generated name)

## Next Steps

1. ✅ Update StorageClass examples with real datacenter IDs
2. ✅ Add datacenter discovery logging to controller
3. ✅ Test volume creation with real datacenter
4. ✅ Fix API field names in client
5. ⏳ Document valid volume sizes (need volume configs endpoint)
6. ⏳ Test volume attach/detach (requires VM)
7. ⏳ Test volume resize operation
