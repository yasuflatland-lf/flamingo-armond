output "service_id" {
  description = "Render service ID."
  value       = render_web_service.this.id
}

output "service_url" {
  description = "Public URL Render assigns to the service. Used as Vercel BACKEND_URL."
  value       = render_web_service.this.url
}
