variable "project_id" {
  description = "GCP project ID"
  type        = string
  default     = "not-scrabble"
}

variable "region" {
  description = "GCP region for Cloud Run and Artifact Registry"
  type        = string
  default     = "us-central1"
}

variable "image_tag" {
  description = "Container image tag to deploy (e.g. 'latest' or a git SHA)"
  type        = string
  default     = "latest"
}
