# Dynamic DNS for Cloudflare hosted zones

This tool will monitor the public IP address of your router, and if it detects that the IP address differs from a specified dns record (hosted in a cloudflare zone), it will use the cloudflare API to update it.
