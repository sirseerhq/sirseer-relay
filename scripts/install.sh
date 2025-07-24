#!/usr/bin/env bash
#
# SirSeer Relay Installation Script
# 
# This script downloads and installs the latest version of sirseer-relay
# for your platform.
#
# Usage:
#   curl -sSL https://raw.githubusercontent.com/sirseerhq/sirseer-relay/main/scripts/install.sh | bash
#   
# Or download and run:
#   ./install.sh [version]
#

set -e

# Configuration
REPO="sirseerhq/sirseer-relay"
BINARY_NAME="sirseer-relay"
INSTALL_DIR="${INSTALL_DIR:-/usr/local/bin}"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Helper functions
info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1"
    exit 1
}

# Detect OS and architecture
detect_platform() {
    OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    ARCH=$(uname -m)

    case $OS in
        linux)
            OS="linux"
            ;;
        darwin)
            OS="darwin"
            ;;
        mingw*|msys*|cygwin*)
            OS="windows"
            ;;
        *)
            error "Unsupported operating system: $OS"
            ;;
    esac

    case $ARCH in
        x86_64|amd64)
            ARCH="amd64"
            ;;
        arm64|aarch64)
            if [ "$OS" = "darwin" ]; then
                ARCH="arm64"
            else
                error "ARM64 is only supported on macOS"
            fi
            ;;
        *)
            error "Unsupported architecture: $ARCH"
            ;;
    esac

    PLATFORM="${OS}-${ARCH}"
    info "Detected platform: $PLATFORM"
}

# Get the latest release version from GitHub
get_latest_version() {
    if [ -n "$1" ]; then
        VERSION="$1"
    else
        info "Fetching latest version..."
        VERSION=$(curl -s "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name":' | sed -E 's/.*"([^"]+)".*/\1/')
        
        if [ -z "$VERSION" ]; then
            error "Failed to fetch latest version"
        fi
    fi
    
    info "Version: $VERSION"
}

# Download the binary
download_binary() {
    BINARY_FILE="${BINARY_NAME}-${PLATFORM}"
    if [ "$OS" = "windows" ]; then
        BINARY_FILE="${BINARY_FILE}.exe"
    fi
    
    URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY_FILE"
    TEMP_FILE="/tmp/${BINARY_NAME}-download"
    
    info "Downloading $URL..."
    
    if command -v curl &> /dev/null; then
        curl -L -o "$TEMP_FILE" "$URL" || error "Failed to download binary"
    elif command -v wget &> /dev/null; then
        wget -O "$TEMP_FILE" "$URL" || error "Failed to download binary"
    else
        error "Neither curl nor wget found. Please install one of them."
    fi
    
    # Verify download
    if [ ! -f "$TEMP_FILE" ]; then
        error "Download failed - file not found"
    fi
    
    # Download and verify checksum
    info "Verifying checksum..."
    CHECKSUM_URL="${URL}.sha256"
    CHECKSUM_FILE="/tmp/${BINARY_NAME}-checksum"
    
    if command -v curl &> /dev/null; then
        curl -L -o "$CHECKSUM_FILE" "$CHECKSUM_URL" || warn "Failed to download checksum"
    else
        wget -O "$CHECKSUM_FILE" "$CHECKSUM_URL" || warn "Failed to download checksum"
    fi
    
    if [ -f "$CHECKSUM_FILE" ]; then
        # Extract just the hash from the checksum file
        EXPECTED_HASH=$(awk '{print $1}' "$CHECKSUM_FILE")
        
        # Calculate actual hash
        if command -v sha256sum &> /dev/null; then
            ACTUAL_HASH=$(sha256sum "$TEMP_FILE" | awk '{print $1}')
        elif command -v shasum &> /dev/null; then
            ACTUAL_HASH=$(shasum -a 256 "$TEMP_FILE" | awk '{print $1}')
        else
            warn "Cannot verify checksum - no sha256sum or shasum found"
            ACTUAL_HASH=""
        fi
        
        if [ -n "$ACTUAL_HASH" ] && [ "$EXPECTED_HASH" != "$ACTUAL_HASH" ]; then
            error "Checksum verification failed"
        elif [ -n "$ACTUAL_HASH" ]; then
            info "Checksum verified"
        fi
        
        rm -f "$CHECKSUM_FILE"
    else
        warn "Could not download checksum file"
    fi
}

# Install the binary
install_binary() {
    # Check if we need sudo
    if [ -w "$INSTALL_DIR" ]; then
        SUDO=""
    else
        SUDO="sudo"
        warn "Installation requires sudo privileges"
    fi
    
    info "Installing to $INSTALL_DIR/$BINARY_NAME..."
    
    # Make executable
    chmod +x "$TEMP_FILE"
    
    # Move to install directory
    $SUDO mv "$TEMP_FILE" "$INSTALL_DIR/$BINARY_NAME"
    
    # Verify installation
    if command -v "$BINARY_NAME" &> /dev/null; then
        INSTALLED_VERSION=$("$BINARY_NAME" --version 2>&1 | head -n1)
        info "Successfully installed: $INSTALLED_VERSION"
    else
        warn "Binary installed but not found in PATH"
        info "You may need to add $INSTALL_DIR to your PATH"
    fi
}

# Main installation flow
main() {
    echo "SirSeer Relay Installer"
    echo "======================"
    echo
    
    detect_platform
    get_latest_version "$1"
    download_binary
    install_binary
    
    echo
    info "Installation complete!"
    echo
    echo "To get started, set your GitHub token:"
    echo "  export GITHUB_TOKEN=ghp_your_token_here"
    echo
    echo "Then fetch pull requests from a repository:"
    echo "  $BINARY_NAME fetch owner/repo"
    echo
    echo "For more information, visit: https://github.com/$REPO"
}

# Run main function
main "$@"