#!/usr/bin/env python
import argparse
import sys

import boto3

parser=argparse.ArgumentParser(description='rm VPC on AWS.')
parser.add_argument('vpc-id', help='VPC')
parser.add_argument('--vm', metavar='i-id', help='VM instance')
args = vars(parser.parse_args())

def rm_instances(vpc, ec2):
    client = boto3.client('ec2')
    response = client.describe_instances(Filters=[
        {"Name": "vpc-id", "Values": [vpc.id]}
    ])
    vms = []
    for r in response['Reservations']:
        for i in r['Instances']:
            if args['vm'] != None:
                if i['InstanceId'] == args['vm']:
                    vms.append(i['InstanceId'])
            else:
                vms.append(i['InstanceId'])
    if vms == []:
        print("There is no instance in this VPC to terminate")
    else:
        print("Terminate: ", vms)
        ec2.instances.filter(InstanceIds = vms).terminate()

    if vms != []:
        waiter = client.get_waiter('instance_terminated')
        waiter.wait(InstanceIds = vms)

def rm_net(vpc, ec2client):
    for rt in vpc.route_tables.all():
        for rta in rt.associations:
            if not rta.main:
                print("delete rt", rt.id)
                rta.delete()
        
    for ep in ec2client.describe_vpc_endpoints(
            Filters=[{
                'Name': 'vpc-id',
                'Values': [vpc.id]
            }])['VpcEndpoints']:
        print("Delete endpoint", ep['VpcEndpointId'])
        ec2client.delete_vpc_endpoints(VpcEndpointIds=[ep['VpcEndpointId']])                   
    for netacl in vpc.network_acls.all():
        if not netacl.is_default:
            print("Delete nacl", netacl.id)
            netacl.delete()

    for sn in vpc.subnets.all():
        print("Delete sn", sn.id, "-", sn.cidr_block)
        sn.delete()

    for gw in vpc.internet_gateways.all():
        vpc.detach_internet_gateway(InternetGatewayId=gw.id)
        print("Delete gw", gw.id)
        gw.delete()

def rm_sec(vpc):
    for sg in vpc.security_groups.all():
        if sg.group_name != 'default':
            print("Delete sg", sg.id)
            sg.delete()

def main():
    vpc_id = args['vpc-id']
    boto3.setup_default_session(profile_name='me-mit')
    ec2 = boto3.resource('ec2')
    ec2client = ec2.meta.client

    try:
        vpc = ec2.Vpc(vpc_id)
        print("VPC CIDR:", vpc.cidr_block)
        rm_instances(vpc, ec2)
        if args['vm'] == None:
            rm_db(vpc)
            rm_net(vpc, ec2client)
            rm_sec(vpc)
            ec2client.delete_vpc(VpcId=vpc.id)
    except Exception as e:
        print("error", e)    

if __name__ == "__main__":
    main()
    
