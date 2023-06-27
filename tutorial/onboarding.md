# Onboarding tasks

This file describes tasks required to onboard new members of the SigmaOS
development team.

## Tasks for existing team members

The following tasks must be completed by an existing team member in order to
onboard a new team member:

  1. Encrypt the AWS and DockerHub credentials for the new team member, by
  running (with a full list of recipients):

```
$ cd aws/.aws
$ gpg --recipient sigma-kaashoek --recipient arielck --recipient heyizheng2011 --recipient gideon.witchel@gmail.com --encrypt-files credentials
$ cd ../.docker
$ gpg --recipient sigma-kaashoek --recipient arielck --recipient heyizheng2011 --recipient gideon.witchel@gmail.com --encrypt-files config.json
```

  2. Add the member to the `git@g.csail.mit.edu:ulambda:` repo and the to the
  [GitHub repo](https://github.com/mit-pdos/sigmaos).

## Tasks for new team members

The following tasks must be completed by new team members during onboarding.

  1. Clone the `sigmaos` git repo from `g`, and rename the directory `sigmaos`
  (the repo is still called `ulambda` on `g` for historical reasons.

```
git clone git@g.csail.mit.edu:ulambda sigmaos
```

  2. Create a gpg key and send it to someone on the development team. GitHub
  has a good guide
  [here](https://docs.github.com/en/authentication/managing-commit-signature-verification/generating-a-new-gpg-key).
  3. Complete the
  [tutorial](https://github.com/mit-pdos/sigmaos/tree/master/tutorial).
