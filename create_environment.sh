#!/bin/bash
# Update package lists
apt-get install -y software-properties-common zlib1g-dev

# Step 3: Add the deadsnakes PPA (contains newer Python versions)
add-apt-repository -y ppa:deadsnakes/ppa

# Step 4: Update package list after adding the new PPA
apt-get update

# Step 5: Install Python 3.9
apt-get install -y python3.9 python3.9-distutils

# Install necessary packages
apt-get -y install unzip zip procps build-essential python3-pip net-tools curl git tmux

# Install Rust
curl https://sh.rustup.rs -sSf | bash -s -- -y

# Source Rust environment variables
source "$HOME/.cargo/env"

# Install wget
apt install wget -y

# Download and install Go
wget https://go.dev/dl/go1.22.6.linux-amd64.tar.gz
rm -rf /usr/local/go && tar -C /usr/local -xzf go1.22.6.linux-amd64.tar.gz
rm go1.22.6.linux-amd64.tar.gz
# Set PATH for Go binaries
export PATH="/usr/local/go/bin:${PATH}"

# Persist the Go PATH for future sessions
echo "export PATH=/usr/local/go/bin:\${PATH}" >> ~/.profile

# Upgrade pip
pip3 install --upgrade pip

# Install Python packages
pip3 install pyopenssl setuptools-rust semantic_version pipenv

# Copy the project files to the desired location
mkdir -p /app
cd /app
git clone --depth=1 https://github.com/glmquint/fedlang.git
cd /app/fedlang
git clone --depth=1 https://github.com/Pyrlang/Pyrlang.git && git clone --depth=1 https://github.com/Pyrlang/Term.git
# Set the working directory

# Install dependencies with pipenv
pipenv -v install
pipenv -v install pandas numba

# Set PATH for Rust binaries
export PATH="/root/.cargo/bin:${PATH}"

# Move to the Term directory and install
cd /app/fedlang/Term
pipenv -v install --deploy --ignore-pipfile
pipenv run pip install .

# Move to the Pyrlang directory and install
cd /app/fedlang/Pyrlang
pipenv -v install --deploy --ignore-pipfile
pipenv run pip install .

# Move back to the main directory and make the start.sh script executable
cd /app/fedlang/
chmod +x start.sh