Since a custom logic can be implemented:

- Labeling is nice to have, it would simplify the scan for resources by app
- Failure tracked into Jira
- Creationtime and Resolution Time are required fields.
- Read access to clusters resources, Jira and github
- Onboarding instructions for existing in-cluster prometheus instance

The sample linked above uses this logic:
Commit time:
- List all argo-cd applications in the openshift-gitops deployment
- For each application list all images in the app.Status.Summary.Images field and get those from quay.io/redhat-appstudio organization
- Get the commit hash from the image name
- Search for that commit on github to pull timestamp data
- Get the component name from ​​.Spec.Source.Path, stripping the “component/” string
- Get the destination namespace
- Get the image name
Deploy Time
- List all deployments by label 
- Filter only deployments with active condition set to true
- Get the deployment CreationTimestamp
- Get Image name
- Get namespace
- Get commit hash
- Get app name from app.kubernetes.io/instance label
Failures (TDB, not implemented yet in the sample code):
- All jira failures must be uniquely searchable with JQL and uniquely identify the application the failures relate to, the failure starting time, the failure ending time.
Suggested approach: label the issues for quick search; populate creationTime and resolutionTime; a failure is considered resolved when the issue is marked as Closed (or Resolved, or Done, or any of that) 