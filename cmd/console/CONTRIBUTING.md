# Contributing to Inrun Console

First off, thank you for considering contributing to the Inrun Console! 🎉

## Project Status

This is a **demonstration UI** built to showcase the capabilities of Inrun. It works and is production-ready, but we have plans to make it a **standard, feature-complete** console. Your contributions can help shape its future!

## How Can I Contribute?

### Report Bugs

If you find a bug, please create an issue with:
- A clear description of the problem
- Steps to reproduce
- Expected vs actual behavior
- Screenshots if applicable
- Your environment (OS, browser, Inrun version)

### Suggest Features

We're actively looking for contributions in these areas:

#### High Priority
- [ ] **User authentication** – Basic auth, OAuth2, SSO integration
- [ ] **Role-based access control** – Team-based permissions
- [ ] **Time-series graphs** – Historical metrics visualization
- [ ] **Alerting** – Email/Slack/PagerDuty integration for degraded CRDs
- [ ] **Dark mode** – Because everyone wants it

#### Medium Priority
- [ ] **Export data** – CSV/JSON export of metrics and health data
- [ ] **Custom dashboards** – Save custom views and filters
- [ ] **WebSocket updates** – Real-time updates without page refresh
- [ ] **Multi-language support** – i18n/l10n
- [ ] **Mobile responsive improvements**

#### Nice to Have
- [ ] **Blueprint diff view** – Compare Blueprints across environments
- [ ] **Audit logging** – Track who viewed what and when
- [ ] **Bookmarkable state** – Save filter/search in URL
- [ ] **Keyboard shortcuts** – Power user navigation

### Improve Documentation

Documentation is always welcome:
- Fix typos or unclear sections
- Add usage examples
- Translate to other languages
- Write tutorials

### Write Code

#### Getting Started

1. Fork the repository
2. Clone your fork:
   ```bash
   git clone https://github.com/your-username/inrun-console.git
   cd inrun-console
   ```
3. Install dependencies:
   ```bash
   go mod download
   ```
4. Create a branch:
   ```bash
   git checkout -b feature/your-feature-name
   ```
5. Make your changes
6. Run tests:
   ```bash
   go test ./...
   ```
7. Build and test locally:
   ```bash
   go build -o inrun-console .
   ./inrun-console -u "http://localhost:8080"
   ```
8. Commit and push:
   ```bash
   git commit -m "feat: add your feature"
   git push origin feature/your-feature-name
   ```
9. Open a Pull Request

#### Code Style

- Follow standard Go conventions (`gofmt`, `go vet`, `golint`)
- Use meaningful variable names
- Add comments for exported functions
- Keep templates clean and well-indented
- Test your changes with multiple Inrun instances

#### Template Guidelines

When modifying HTML templates:
- Use TailwindCSS utility classes
- Ensure responsive design works on mobile
- Test in at least Chrome and Firefox
- Keep JavaScript minimal and well-commented

## Development Environment

### Running with Multiple Test Instances

```bash
# Terminal 1 – Inrun Runtime 1
export INRUN_PORT=8080
inrun -f blueprint1.yaml

# Terminal 2 – Inrun Runtime 2
export INRUN_PORT=8081
inrun -f blueprint2.yaml

# Terminal 3 – Console
cd console
go run main.go -u "http://localhost:8080,http://localhost:8081" -p 8081
```

### Debug Mode

```bash
# Run console with debug logging
./inrun-console -u "http://localhost:8080" -log-level debug

# Check cached instances
curl http://localhost:8081/console/debug/instances
```

## Pull Request Process

1. Update the README.md with details of changes if needed
2. Update the CHANGELOG.md (if significant)
3. The PR will be merged once reviewed and approved
4. Your name will be added to CONTRIBUTORS.md

## Code of Conduct

- Be respectful and inclusive
- Provide constructive feedback
- Focus on what's best for the community
- No harassment or offensive behavior

## Recognition

Contributors will be recognized in:
- GitHub contributors list
- Release notes
- (Future) Hall of Fame in documentation

## Questions?

- Open an issue for bugs or feature requests
- Join our discussions
- Email the maintainers

## Thank You!

Your contributions make Inrun better for everyone. Whether you're fixing a typo, adding a feature, or improving documentation, we appreciate you! 🙏

---

*Inrun Console – Part of the Orkesta Project*
*Apache licensed*
