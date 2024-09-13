#!/usr/bin/env python
import argparse

import boto3

parser=argparse.ArgumentParser(description='ls vpc on AWS.')
parser.add_argument('vpc-id', help='VPC')
parser.add_argument('--privaddr', dest='privaddr', action='store_true', help='Private IP Address')
args = vars(parser.parse_args())

def ls_nets(vpc):
    subnets = list(vpc.subnets.all())
    if len(subnets) > 0:
        for sn in subnets:
            print("Subnet:", sn.id, "-", sn.cidr_block)
    else:
        print("There is no subnet in this VPC")

def ls_sg(vpc):
    for sg in vpc.security_groups.all():
        if sg.group_name != 'default':
            print("Security group:", sg.id, sg.group_name)

def cmp(vm):
  if "sigma" not in name(vm[3]) and "k8s" not in name(vm[3]):
    return 99999
  return int(name(vm[3]).replace("sigma", "").replace("k8s",""))

def name(tags):
    name = ""
    for d in tags:
        if d['Key'] == 'Name':
            name = d['Value']
    return name

def ls_instances(vpc):
    client = boto3.client('ec2')
    response = client.describe_instances(Filters=[
        {"Name": "vpc-id", "Values": [vpc.id]}
    ])
    vms = []
    for r in response['Reservations']:
        for i in r['Instances']:
            if 'Tags' in i:
                vms.append((i['InstanceId'], i['PublicDnsName'], i['PrivateIpAddress'], i['Tags']))
            else:
                vms.append((i['InstanceId'], i['PublicDnsName'], i['PrivateIpAddress'], []))
    if vms == []:
        print("There is no instance in this VPC")
    else:
        vms.sort(key=cmp)
        for vm in vms:
            if "student-dev" in name(vm[3]):
              continue
            if args['privaddr']:
                print("VMInstance", name(vm[3]), ":", vm[0], vm[1], vm[2])
            else:
                print("VMInstance", name(vm[3]), ":", vm[0], vm[1])

def main():
   vpc_id = args['vpc-id']

   boto3.setup_default_session(profile_name='sigmaos')
   ec2 = boto3.resource('ec2')

   try:
       vpc = ec2.Vpc(vpc_id)
       print("VPC Name:", name(vpc.tags))
       ls_nets(vpc)
       ls_sg(vpc)
       ls_instances(vpc)

   except Exception as e:
       print("error", e)


if __name__ == "__main__":
    main()
    
