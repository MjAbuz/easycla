/**
 * Creates basic security groups to be used by instances and ELBs.
 */

variable "name" {
  description = "The name of the security groups serves as a prefix, e.g stack"
}

variable "vpc_id" {
  description = "The VPC ID"
}

variable "cidr" {
  description = "The cidr block to use for internal security groups"
}

resource "aws_security_group" "pypi_redis" {
  provider    = "aws.local"
  name        = "${format("%s-redis-server", var.name)}"
  description = "Redis Server for Pypi"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    cidr_blocks     = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name        = "${format("%s redis for pypi", var.name)}"
  }
}

resource "aws_security_group" "vpn" {
  provider    = "aws.local"
  name        = "${format("%s-vpn", var.name)}"
  description = "Pritunl OpenVPN Server"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port   = 10000
    to_port     = 20000
    protocol    = "udp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 443
    to_port     = 443
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["10.32.0.0/12"]
  }

  ingress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    security_groups = ["${aws_security_group.jenkins-master.id}"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name        = "Pritunl OpenVPN Server"
  }
}

resource "aws_security_group" "jenkins-slaves" {
  provider    = "aws.local"
  name        = "jenkins-slaves"
  description = "Jenkins Slaves"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    cidr_blocks     = ["10.32.0.0/12"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags {
    Name        = "Jenkins Slave Instances"
  }
}

resource "aws_security_group" "jenkins-master" {
  provider    = "aws.local"
  name        = "jenkins-master"
  description = "Jenkins Master Instance"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    cidr_blocks     = ["10.32.0.0/12"]
  }

  ingress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    security_groups = ["${aws_security_group.jenkins-slaves.id}"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags {
    Name        = "Jenkins Master Instance"
  }
}

resource "aws_security_group" "jenkins-master-efs" {
  provider    = "aws.local"
  name        = "jenkins-master-efs"
  description = "Allows Internal EFS use from Jenkins Master"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port       = 2049
    to_port         = 2049
    protocol        = "tcp"
    security_groups = ["${aws_security_group.jenkins-master.id}"]
  }

  egress {
    from_port   = 2049
    to_port     = 2049
    protocol    = "tcp"
    cidr_blocks = ["0.0.0.0/0"]
  }

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name        = "Jenkins Master Instance EFS"
  }
}

resource "aws_security_group" "engineering-sandboxes-elb" {
  provider    = "aws.local"
  name        = "engineering-sandboxes-elb"
  description = "Allows Internal EFS use from Jenkins Master"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port       = 80
    to_port         = 80
    protocol        = "tcp"
    cidr_blocks     = ["0.0.0.0/0"]
  }

  ingress {
    from_port       = 443
    to_port         = 443
    protocol        = "tcp"
    cidr_blocks     = ["0.0.0.0/0"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  lifecycle {
    create_before_destroy = true
  }

  tags {
    Name        = "Engineering Sandboxes ELB"
  }
}

resource "aws_security_group" "engineering-sandboxes" {
  provider    = "aws.local"
  name        = "engineering-sandboxes"
  description = "Engineering Sandboxes"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    security_groups = ["${aws_security_group.engineering-sandboxes-elb.id}"]
  }

  ingress {
    from_port       = 0
    to_port         = 0
    protocol        = "-1"
    security_groups = ["${aws_security_group.engineering-sandboxes-redis.id}"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags {
    Name        = "Engineering Sandboxes"
  }
}

resource "aws_security_group" "engineering-sandboxes-redis" {
  provider    = "aws.local"
  name        = "engineering-sandboxes-redis"
  description = "Engineering Sandboxes Redis"
  vpc_id      = "${var.vpc_id}"

  ingress {
    from_port       = 6379
    to_port         = 6379
    protocol        = "tcp"
    cidr_blocks     = ["${var.cidr}"]
  }

  egress {
    from_port   = 0
    to_port     = 0
    protocol    = "-1"
    cidr_blocks = ["0.0.0.0/0"]
  }

  tags {
    Name        = "Engineering Sandboxes Redis Server"
  }
}

output "pypi_redis" {
  value = "${aws_security_group.pypi_redis.id}"
}

output "vpn" {
  value = "${aws_security_group.vpn.id}"
}

output "jenkins_slaves" {
  value = "${aws_security_group.jenkins-slaves.id}"
}

output "jenkins_master" {
  value = "${aws_security_group.jenkins-master.id}"
}

output "jenkins_master_efs" {
  value = "${aws_security_group.jenkins-master-efs.id}"
}

output "engineering_sandboxes" {
  value = "${aws_security_group.engineering-sandboxes.id}"
}

output "engineering_sandboxes_elb" {
  value = "${aws_security_group.engineering-sandboxes-elb.id}"
}

output "engineering_sandboxes_redis" {
  value = "${aws_security_group.engineering-sandboxes-redis.id}"
}