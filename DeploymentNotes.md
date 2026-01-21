# Deployment Guide - GCP Compute Engine

## Prerequisites

- Google Cloud Platform account
- `gcloud` CLI installed and authenticated
- Docker installed locally

## Step 1: Build and Push Image

```bash
# Set your project ID
PROJECT_ID="your-gcp-project-id"

# Build for GCP
docker build -t gcr.io/$PROJECT_ID/redis-clone .

# Authenticate with GCR
gcloud auth configure-docker

# Push image
docker push gcr.io/$PROJECT_ID/redis-clone
```

## Step 2: Deploy to Compute Engine

```bash
# Create VM with your container
gcloud compute instances create-with-container redis-server \
  --container-image=gcr.io/$PROJECT_ID/redis-clone \
  --container-restart-policy=always \
  --zone=us-central1-a \
  --machine-type=e2-micro \
  --boot-disk-size=10GB \
  --boot-disk-type=pd-standard \
  --tags=redis-server

# Open firewall for Redis port
gcloud compute firewall-rules create allow-redis \
  --allow=tcp:6379 \
  --target-tags=redis-server \
  --source-ranges=0.0.0.0/0
```

## Step 3: Get Connection Details

```bash
# Get external IP
gcloud compute instances describe redis-server \
  --zone=us-central1-a \
  --format='get(networkInterfaces[0].accessConfigs[0].natIP)'

# Output example: 34.123.45.67
```

## Step 4: Connect from Your App

**From Go application:**

```go
conn, err := net.Dial("tcp", "34.123.45.67:6379")
```

**Using redis-cli:**

```bash
redis-cli -h 34.123.45.67 -p 6379
```

## Step 5: Persist AOF Across Restarts (Optional)

```bash
# Create persistent disk
gcloud compute disks create redis-data \
  --size=10GB \
  --zone=us-central1-a

# Attach to instance
gcloud compute instances attach-disk redis-server \
  --disk=redis-data \
  --zone=us-central1-a

# SSH into instance
gcloud compute ssh redis-server --zone=us-central1-a

# Format and mount disk
sudo mkfs.ext4 /dev/sdb
sudo mkdir -p /mnt/redis-data
sudo mount /dev/sdb /mnt/redis-data
sudo chown -R 1000:1000 /mnt/redis-data

# Update container to use mounted volume
# (Requires recreating instance with volume mount)
```

## Management Commands

**View logs:**

```bash
gcloud compute ssh redis-server --zone=us-central1-a
docker logs $(docker ps -q)
```

**Restart container:**

```bash
gcloud compute ssh redis-server --zone=us-central1-a
docker restart $(docker ps -q)
```

**Stop instance:**

```bash
gcloud compute instances stop redis-server --zone=us-central1-a
```

**Delete instance:**

```bash
gcloud compute instances delete redis-server --zone=us-central1-a
```
