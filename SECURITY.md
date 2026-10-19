# Security Policy

## Supported Versions

Inrun is currently in active development.  
Security fixes will be applied to the latest minor release.

| Version | Supported |
|---------|-----------|
| v0.x.x  | ✔ Active |
| < v0.x  | ✖ Unsupported |

## Reporting a Vulnerability

If you discover a security vulnerability, please report it privately.

Use [GitHub private security advisories](https://github.com/intentd/inrun/security/advisories/new) to report privately.

Please include:

- A detailed description of the issue  
- Steps to reproduce  
- Potential impact  
- Any suggested fixes  

We will acknowledge your report within **72 hours**.

## Disclosure Policy

- We follow **responsible disclosure** practices.  
- We will work with you to validate and fix the issue.  
- We will publish a security advisory once a fix is available.  
- You will be credited unless you request anonymity.

## What Is Considered a Security Issue?

- Privilege escalation  
- Unauthorized resource access  
- CRD or cluster‑wide compromise  
- Remote code execution  
- Bypass of Inrun’s isolation guarantees  

## What Is *Not* Considered a Security Issue?

- Misconfigured RBAC in user clusters  
- Incorrect Blueprint definitions  
- Expected Kubernetes behavior  
- Resource exhaustion caused by user‑defined CRDs  
