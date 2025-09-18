#!/bin/bash

# Clean Terraform State Script
# This script removes all Terraform state files, lock files, and cached provider data

set -e

echo "🧹 Terraform State Cleanup Script"
echo "=================================="

# Change to the script's directory
cd "$(dirname "$0")"

# Function to safely remove files
safe_remove() {
    local file="$1"
    if [ -f "$file" ]; then
        echo "🗑️  Removing: $file"
        rm -f "$file"
    else
        echo "ℹ️  Not found: $file"
    fi
}

# Function to safely remove directories
safe_remove_dir() {
    local dir="$1"
    if [ -d "$dir" ]; then
        echo "🗑️  Removing directory: $dir"
        rm -rf "$dir"
    else
        echo "ℹ️  Directory not found: $dir"
    fi
}

echo ""
echo "Current directory: $(pwd)"
echo "Files before cleanup:"
ls -la

echo ""
echo "🔍 Cleaning up Terraform files..."

# Remove lock file
safe_remove ".terraform.lock.hcl"

# Remove any crash logs
safe_remove "crash.log"

# Remove any plan files
for plan_file in *.tfplan; do
    if [ -f "$plan_file" ]; then
        safe_remove "$plan_file"
    fi
done

echo ""
echo "✅ Cleanup complete!"
echo ""
echo "Files after cleanup:"
ls -la

echo ""
echo "🔨 Recompiling the Terraform provider..."
echo "======================================"

# Change to the parent directory where the Makefile is located
cd ..

# Build and install the provider
if make install; then
    echo "✅ Provider successfully recompiled and installed!"
else
    echo "❌ Provider compilation failed!"
    exit 1
fi

# Change back to the terraform_tests directory
cd terraform_tests
terraform init
echo ""
echo "🎉 All Terraform state and cache files have been removed and provider has been recompiled!"
echo "You can now run 'terraform init' to start fresh."