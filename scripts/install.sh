#!/bin/bash
set -e

# Emma CSI Driver Installation Script
# This script installs the Emma CSI Driver in your Kubernetes cluster

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Print functions
print_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

print_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

print_header() {
    echo ""
    echo "=========================================="
    echo "$1"
    echo "=========================================="
    echo ""
}

# Check prerequisites
check_prerequisites() {
    print_header "Checking Prerequisites"
    
    # Check kubectl
    if ! command -v kubectl &> /dev/null; then
        print_error "kubectl not found. Please install kubectl first."
        exit 1
    fi
    print_info "✓ kubectl found: $(kubectl version --client --short 2>/dev/null || kubectl version --client)"
    
    # Check cluster access
    if ! kubectl cluster-info &> /dev/null; then
        print_error "Cannot access Kubernetes cluster. Please configure kubectl."
        exit 1
    fi
    print_info "✓ Kubernetes cluster accessible"
    
    # Check cluster version
    K8S_VERSION=$(kubectl version --short 2>/dev/null | grep Server | awk '{print $3}' | sed 's/v//' || kubectl version -o json | grep -o '"gitVersion": "v[^"]*"' | head -1 | sed 's/.*v\([0-9.]*\).*/\1/')
    MAJOR=$(echo $K8S_VERSION | cut -d. -f1)
    MINOR=$(echo $K8S_VERSION | cut -d. -f2)
    
    if [ "$MAJOR" -lt 1 ] || ([ "$MAJOR" -eq 1 ] && [ "$MINOR" -lt 20 ]); then
        print_error "Kubernetes version 1.20+ required. Found: $K8S_VERSION"
        exit 1
    fi
    print_info "✓ Kubernetes version: $K8S_VERSION"
}

# Prompt for Emma credentials
get_credentials() {
    print_header "Emma API Credentials"
    
    echo "You need Emma.ms API credentials to proceed."
    echo "If you don't have them yet:"
    echo "  1. Log in to https://portal.emma.ms"
    echo "  2. Go to Settings > Service Applications"
    echo "  3. Create a new Service Application with 'Manage' access"
    echo "  4. Copy the Client ID and Client Secret"
    echo ""
    
    # Check if credentials already exist
    if kubectl get secret emma-api-credentials -n kube-system &> /dev/null; then
        print_warn "Secret 'emma-api-credentials' already exists in kube-system namespace."
        read -p "Do you want to update it? (y/N): " -n 1 -r
        echo
        if [[ ! $REPLY =~ ^[Yy]$ ]]; then
            print_info "Using existing credentials."
            return
        fi
        kubectl delete secret emma-api-credentials -n kube-system
    fi
    
    # Prompt for credentials
    read -p "Enter Emma Client ID: " CLIENT_ID
    read -sp "Enter Emma Client Secret: " CLIENT_SECRET
    echo ""
    
    if [ -z "$CLIENT_ID" ] || [ -z "$CLIENT_SECRET" ]; then
        print_error "Client ID and Client Secret are required."
        exit 1
    fi
    
    # Create secret
    print_info "Creating Kubernetes secret..."
    kubectl create secret generic emma-api-credentials \
        --from-literal=client-id="$CLIENT_ID" \
        --from-literal=client-secret="$CLIENT_SECRET" \
        --namespace=kube-system
    
    print_info "✓ Secret created successfully"
}

# Deploy CSI driver
deploy_driver() {
    print_header "Deploying Emma CSI Driver"
    
    # Get script directory
    SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" && pwd )"
    DEPLOY_DIR="$SCRIPT_DIR/../deploy"
    
    if [ ! -d "$DEPLOY_DIR" ]; then
        print_error "Deploy directory not found: $DEPLOY_DIR"
        exit 1
    fi
    
    # Check if kustomize is available
    if kubectl kustomize --help &> /dev/null; then
        print_info "Deploying using kustomize..."
        kubectl apply -k "$DEPLOY_DIR"
    else
        print_info "Deploying using individual manifests..."
        kubectl apply -f "$DEPLOY_DIR/csidriver.yaml"
        kubectl apply -f "$DEPLOY_DIR/rbac.yaml"
        kubectl apply -f "$DEPLOY_DIR/configmap.yaml"
        kubectl apply -f "$DEPLOY_DIR/controller.yaml"
        kubectl apply -f "$DEPLOY_DIR/node.yaml"
        kubectl apply -f "$DEPLOY_DIR/storageclass.yaml"
    fi
    
    print_info "✓ Manifests applied"
}

# Wait for deployment
wait_for_deployment() {
    print_header "Waiting for Deployment"
    
    print_info "Waiting for controller to be ready..."
    kubectl wait --for=condition=ready pod \
        -l app=emma-csi-controller \
        -n kube-system \
        --timeout=300s || {
        print_error "Controller failed to become ready. Check logs with:"
        print_error "  kubectl logs -n kube-system -l app=emma-csi-controller"
        exit 1
    }
    print_info "✓ Controller is ready"
    
    print_info "Waiting for node plugins to be ready..."
    kubectl wait --for=condition=ready pod \
        -l app=emma-csi-node \
        -n kube-system \
        --timeout=300s || {
        print_error "Node plugins failed to become ready. Check logs with:"
        print_error "  kubectl logs -n kube-system -l app=emma-csi-node"
        exit 1
    }
    print_info "✓ Node plugins are ready"
}

# Verify installation
verify_installation() {
    print_header "Verifying Installation"
    
    # Check CSI driver
    if kubectl get csidriver csi.emma.ms &> /dev/null; then
        print_info "✓ CSI driver registered"
    else
        print_error "CSI driver not registered"
        exit 1
    fi
    
    # Check controller
    CONTROLLER_READY=$(kubectl get pods -n kube-system -l app=emma-csi-controller -o jsonpath='{.items[0].status.conditions[?(@.type=="Ready")].status}')
    if [ "$CONTROLLER_READY" = "True" ]; then
        print_info "✓ Controller pod is running"
    else
        print_error "Controller pod is not ready"
        exit 1
    fi
    
    # Check node plugins
    NODE_COUNT=$(kubectl get nodes -o json | jq '.items | length')
    NODE_PLUGIN_COUNT=$(kubectl get pods -n kube-system -l app=emma-csi-node -o json | jq '.items | length')
    
    if [ "$NODE_PLUGIN_COUNT" -eq "$NODE_COUNT" ]; then
        print_info "✓ Node plugins running on all $NODE_COUNT nodes"
    else
        print_warn "Expected $NODE_COUNT node plugins, found $NODE_PLUGIN_COUNT"
    fi
    
    # Check storage classes
    SC_COUNT=$(kubectl get storageclass -l app.kubernetes.io/name=emma-csi-driver -o json | jq '.items | length')
    print_info "✓ $SC_COUNT StorageClasses created"
}

# Print next steps
print_next_steps() {
    print_header "Installation Complete!"
    
    echo "The Emma CSI Driver has been successfully installed."
    echo ""
    echo "Next steps:"
    echo ""
    echo "1. Test with a sample PVC:"
    echo "   kubectl apply -f - <<EOF"
    echo "   apiVersion: v1"
    echo "   kind: PersistentVolumeClaim"
    echo "   metadata:"
    echo "     name: test-pvc"
    echo "   spec:"
    echo "     accessModes:"
    echo "       - ReadWriteOnce"
    echo "     storageClassName: emma-ssd"
    echo "     resources:"
    echo "       requests:"
    echo "         storage: 10Gi"
    echo "   EOF"
    echo ""
    echo "2. Check PVC status:"
    echo "   kubectl get pvc test-pvc"
    echo ""
    echo "3. View available StorageClasses:"
    echo "   kubectl get storageclass"
    echo ""
    echo "4. Monitor the driver:"
    echo "   kubectl logs -n kube-system -l app=emma-csi-controller -f"
    echo ""
    echo "For more information, see:"
    echo "  - INSTALL.md - Quick installation guide"
    echo "  - deploy/README.md - Detailed deployment documentation"
    echo "  - docs/USER_GUIDE.md - Usage examples and best practices"
    echo ""
}

# Main installation flow
main() {
    echo ""
    echo "╔════════════════════════════════════════╗"
    echo "║   Emma CSI Driver Installation Script  ║"
    echo "╚════════════════════════════════════════╝"
    echo ""
    
    check_prerequisites
    get_credentials
    deploy_driver
    wait_for_deployment
    verify_installation
    print_next_steps
}

# Run main function
main
