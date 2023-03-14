'''
  (C) Copyright 2023 Intel Corporation.

  SPDX-License-Identifier: BSD-2-Clause-Patent
'''
import os
import random
import time

from ClusterShell.NodeSet import NodeSet

from general_utils import get_random_string, report_errors
from agent_utils import include_local_host
from ior_test_base import IorTestBase
from run_utils import run_remote


# pylint: disable=line-too-long
"""
    TODO - proper place for this
    Steps to setup repo files:
    # Get "old" repo
    clush -B -w $ALL_NODES 'sudo wget -O /etc/yum.repos.d/daos-packages-v2.2.repo https://packages.daos.io/v2.2/EL8/packages/x86_64/daos_packages.repo'

    # Get "new" repo
    clush -B -w $ALL_NODES 'sudo wget -O /etc/yum.repos.d/daos-packages-v2.3.105.repo https://packages.daos.io/private/v2.3.105/EL8/packages/x86_64/daos_packages.repo'

    # "disable" new repo
    clush -B -w $ALL_NODES 'sudo mv /etc/yum.repos.d/daos-packages-v2.3.105.repo /etc/yum.repos.d/daos-packages-v2.3.105.repo_sav'


    # My personal WIP notes
    # To get local avocado to work with RPM install
    cp /usr/lib/daos/.build_vars.* /home/dbohning/daos/install/lib/daos/

    # To install the "old" rpms
    ALL_NODES=boro-24,boro-25
    clush -B -w $ALL_NODES 'sudo systemctl stop daos_agent'; \
    clush -B -w $ALL_NODES 'sudo systemctl stop daos_server'; \
    clush -B -w $ALL_NODES 'sudo yum remove -y daos'; \
    clush -B -w $ALL_NODES 'sudo mv /etc/yum.repos.d/daos-packages-v2.2.repo_sav /etc/yum.repos.d/daos-packages-v2.2.repo'; \
    clush -B -w $ALL_NODES 'sudo mv /etc/yum.repos.d/daos-packages-v2.3.105.repo /etc/yum.repos.d/daos-packages-v2.3.105.repo_sav'; \
    clush -B -w $ALL_NODES 'sudo yum install -y daos-server-tests daos-tests'; \
    clush -B -w $ALL_NODES 'rpm -qa | grep daos | sort'; \
    cp /usr/lib/daos/.build_vars.* /home/dbohning/daos/install/lib/daos/

    # Rebuild just ftest locally
    ~/bin/daos_rebuild_ftest

    # Build IOR locally
    sudo dnf install -y daos-devel
    pushd ~/daos-stack/ior-hpc/
    ./configure --with-mpiio --with-daos=/usr --with-hdf5=/home/dbohning/HDFGroup/install/ --prefix=/home/dbohning/daos-stack/ior-hpc/
    make install
    export PATH=$PATH:$HOME/daos-stack/ior-hpc/bin

    # Run launch.py
    cd ~/daos/install/lib/daos/TESTING/ftest/
    ./launch.py -aro -tc boro-24 -ts boro-25 test_upgrade_downgrade

    # Compare rpms between two nodes
    diff <(clush -B -w boro-24 'rpm -qa | sort' 2>&1) <(clush -B -w boro-25 'rpm -qa | sort' 2>&1)
    
    # TODO launch.py _faults_enabled
"""
# pylint: enable=line-too-long


class UpgradeDowngradeBase(IorTestBase):
    # pylint: disable=too-many-public-methods
    """
    Tests DAOS container attribute get/set/list.
    :avocado: recursive
    """
    def __init__(self, *args, **kwargs):
        """Initialize a ContainerAttributeTest object."""
        super().__init__(*args, **kwargs)
        self.daos_cmd = None
        self.upgrade_repo = ""
        self.downgrade_repo = ""
        self.old_version = ""
        self.new_version = ""
        self.first_client = NodeSet()

    def setUp(self):
        """Set up each test case."""
        super().setUp()
        self.first_client = NodeSet(self.hostlist_clients[0])

    def tearDown(self):
        """Tear down after each test case."""
        # pool destroy --recursive was added in 2.4, so cannot be used in older versions
        if self.container:
            # self.container.destroy()  # TODO
            self.container = None
        if self.pool:
            self.pool.destroy(force=True, recursive=False)
            self.pool = None
        super().tearDown()

    def full_system_stop(self):
        """Stop all servers and agents."""
        self.get_dmg_command().system_stop()
        errors = []
        errors.extend(self._stop_managers(self.agent_managers, "agents"))
        errors.extend(self._stop_managers(self.server_managers, "servers"))
        report_errors(self, errors)
        self.log.info("==sleeping 30 seconds")
        time.sleep(30)

    def get_container(self, pool, namespace=None, create=False, daos_command=None, **kwargs):
        """Create a TestContainer object.

        Overrides base method to handle backward compatibility with labels.

        Args:
            pool (TestPool): pool in which to create the container.
            namespace (str, optional): namespace for TestContainer parameters in
                the test yaml file. Defaults to None.
            create (bool, optional): should the container be created. Defaults to False.
            daos_command (DaosCommand, optional): daos command object.
                Defaults to self.get_daos_command()
            kwargs (dict): name/value of attributes for which to call update(value, name).
                See TestContainer for available attributes.

        Returns:
            TestContainer: the created container.

        Raises:
            AttributeError: if an attribute does not exist or does not have an update() method.

        """
        # Get the params, etc. but do not create the container
        container = super().get_container(pool, namespace, create, daos_command, **kwargs)

        # Adjust the label to be passed in properties
        label = container.label.value
        container.label.update(None)
        container.properties.update(
            ",".join(filter(None, [container.properties.value, f'label:{label}'])))
        container.create()

        # Update local label reference
        container.label.update(label)

        return container

    def pool_set_attrs(self, pool, attrs):
        """Call daos pool set-attr on a client node.

        Handles backward compatibility.

        Args:
            pool (str): pool label
            attrs (dict): attribute name, value pairs
        """
        for attr_name, attr_val in attrs.items():
            cmd = f'daos pool set-attr {pool} "{attr_name}" "{attr_val}"'
            if not run_remote(self.log, self.first_client, cmd).passed:
                self.fail('Failed to set pool attributes')

    def pool_list_attrs(self, pool):
        """Call daos pool list-attrs on a client node.

        Cannot use json output until DAOS-13713 is resolved.

        Args:
            pool (str): pool label

        Returns:
            dict: attribute name, value pairs
        """
        cmd = f'daos pool list-attrs {pool} --verbose'
        result = run_remote(self.log, self.first_client, cmd)
        if not result.passed:
            self.fail('Failed to list pool attributes')
        attrs = {}
        for stdout in result.all_stdout.values():
            for line in stdout.split('\n')[3:]:
                key, val = map(str.strip, line.split(' ', maxsplit=1))
                attrs[key] = val
        return attrs

    @staticmethod
    def create_attr_dict(num_attributes):
        """Create the large attribute dictionary.

        Args:
            num_attributes (int): number of attributes to be created on container.
        Returns:
            dict: a large attribute dictionary
        """
        data_set = {}
        for index in range(num_attributes):
            size = random.randint(1, 10)  # nosec
            attr_name = f'attr{str(index).rjust(2, "0")}'
            attr_val = str(get_random_string(size))
            data_set[attr_name] = attr_val
        return data_set

    def verify_pool_attrs(self, pool, attrs_set):
        """"Verify pool attributes.

        Args:
            pool (TestPool): pool to verify
            attrs_set (dict): expected pool attributes data.
        """
        attrs_list = self.pool_list_attrs(pool.identifier)
        self.log.info("==Verifying list_attr output:")
        self.log.info("  attributes from set-attr:  %s", attrs_set)
        self.log.info("  attributes from list-attr:  %s", attrs_list)
        self.assertEqual(attrs_set, attrs_list, "pool attrs from set-attr do not match list-attr")

    def show_daos_version(self, all_hosts, hosts_client):
        """show daos version

        Args:
            all_hosts (NodeSet): all hosts.
            hosts_client (NodeSet): client hosts to show daos and dmg version.
        """
        if not run_remote(self.log, all_hosts, "rpm -qa | sort | grep daos").passed:
            self.fail("Failed to check daos RPMs")
        if not run_remote(self.log, hosts_client, "dmg version").passed:
            self.fail("Failed to check dmg version")
        if not run_remote(self.log, hosts_client, "daos version").passed:
            self.fail("Failed to check daos version")

    def updowngrade_via_repo(self, servers, clients, repo_1, repo_2):
        """Upgrade or downgrade hosts.

        Args:
            servers (NodeSet): servers to upgrade or downgrade
            clients (NodeSet): clients to upgrade or downgrade
            repo_1 (str): path of the original repository and to be downgraded
            repo_2 (str): path of the new repository to be upgraded
        """
        repo_1_sav = repo_1 + "_sav"
        repo_2_sav = repo_2 + "_sav"
        cmds = [
            "sudo yum remove -y daos",
            f"sudo mv '{repo_1}' '{repo_1_sav}'",
            f"sudo mv '{repo_2_sav}' '{repo_2}'",
            "rpm -qa | sort | grep daos",
            "sudo yum install -y daos-server-tests daos-tests",
            "rpm -qa | sort | grep daos"]
        cmds_client = cmds + [
            # "sudo yum install -y ior",  # TODO
            "sudo cp /etc/daos/daos_agent.yml.rpmsave /etc/daos/daos_agent.yml",
            "sudo cp /etc/daos/daos_control.yml.rpmsave /etc/daos/daos_control.yml"]
        cmds_svr = cmds + [
            "sudo cp /etc/daos/daos_server.yml.rpmsave /etc/daos/daos_server.yml"]

        if servers:
            self.log.info("==upgrade_downgrading on servers: %s", servers)
            for cmd in cmds_svr:
                if not run_remote(self.log, servers, cmd).passed:
                    self.fail("Failed to upgrade/downgrade servers via repo")
            self.log.info("==servers upgrade/downgrade success")
            # (5)Restart servers
            self.log.info("==Restart servers after upgrade/downgrade.")
            self.restart_servers()
        if clients:
            self.log.info("==upgrade_downgrading on hosts_client: %s", clients)
            for cmd in cmds_client:
                if not run_remote(self.log, clients, cmd).passed:
                    self.fail("Failed to upgrade/downgrade clients via repo")
            self.log.info("==clients upgrade/downgrade success")

        self.log.info("==sleeping 5 more seconds after upgrade/downgrade")
        time.sleep(5)

    def upgrade(self, servers, clients):
        """Upgrade hosts via repository or RPMs

        Args:
            servers (NodeSet): servers to be upgraded.
            clients (NodeSet): clients to be upgraded.
        """
        # self.log.info("Skipping upgrade() for now")
        # return
        # pylint: disable=unreachable
        if ".repo" in self.upgrade_repo:
            repo_2 = self.upgrade_repo
            repo_1 = self.downgrade_repo
            self.updowngrade_via_repo(servers, clients, repo_1, repo_2)
        else:
            all_hosts = servers + clients
            self.updowngrade_via_rpms(all_hosts, "upgrade", self.upgrade_repo)

    def downgrade(self, servers, clients):
        """Downgrade hosts via repository or RPMs

        Args:
            servers (NodeSet): servers to be upgraded.
            clients (NodeSet): clients to be upgraded.
        """
        # self.log.info("Skipping downgrade() for now")
        # return
        # pylint: disable=unreachable
        if ".repo" in self.upgrade_repo:
            repo_1 = self.upgrade_repo
            repo_2 = self.downgrade_repo
            self.updowngrade_via_repo(servers, clients, repo_1, repo_2)
        else:
            all_hosts = servers + clients
            self.updowngrade_via_rpms(all_hosts, "downgrade", self.downgrade_repo)

    def updowngrade_via_rpms(self, hosts, updown, rpms):
        """Upgrade downgrade hosts

        Args:
            hosts (NodeSet): test hosts.
            updown (str): upgrade or downgrade
            rpms (list): full path of RPMs to be upgrade or downgrade
        """
        cmds = []
        for rpm in rpms:
            cmds.append("sudo yum {} -y {}".format(updown, rpm))
        cmds.append("sudo ipcrm -a")
        cmds.append("sudo ipcs")
        self.log.info("==%s on hosts: %s", updown, hosts)
        for cmd in cmds:
            if not run_remote(self.log, hosts, cmd).passed:
                self.fail(f"Failed to {updown} via rpms")
        self.log.info("==sleeping 5 more seconds")
        time.sleep(5)
        self.log.info("==%s via rpms success", updown)

    def daos_ver_after_upgraded(self, host):
        """To display daos and dmg version, and check for error.

        Args:
            host (NodeSet): test host.
        """
        cmds = [
            "daos version",
            "dmg version",
            "daos pool query {}".format(self.pool.identifier)]
        for cmd in cmds:
            if not run_remote(self.log, host, cmd).passed:
                self.fail("Failed to get daos and dmg version after upgrade/downgrade")

    def verify_daos_libdaos(self, step, hosts, cmd, positive_test, agent_server_ver, exp_err=None):
        """Verify daos and libdaos interoperability between different version of agent and server.

        Args:
            step (str): test step for logging.
            hosts (NodeSet): hosts to run command on.
            cmd (str): command to run.
            positive_test (bool): True for positive test, false for negative test.
            agent_server_ver (str): agent and server version.
            exp_err (str, optional): expected error message for negative testcase.
                Defaults to None.
        """
        if positive_test:
            self.log.info("==(%s)Positive_test: %s, on %s", step, cmd, agent_server_ver)
        else:
            self.log.info("==(%s)Negative_test: %s, on %s", step, cmd, agent_server_ver)
        result = run_remote(self.log, hosts, cmd)
        if positive_test:
            if not result.passed:
                self.fail("##({0})Test failed, {1}, on {2}".format(step, cmd, agent_server_ver))
        else:
            if result.passed_hosts:
                self.fail("##({0})Test failed, {1}, on {2}".format(step, cmd, agent_server_ver))
            for stdout in result.all_stdout.values():
                if exp_err not in stdout:
                    self.fail("##({0})Test failed, {1}, on {2}, expect_err {3} "
                              "not shown on stdout".format(step, cmd, agent_server_ver, exp_err))

        self.log.info("==(%s)Test passed, %s, on %s", step, cmd, agent_server_ver)

    def has_fault_injection(self, hosts):
        """Check if RPMs with fault-injection function.

        Args:
            hosts (string, list): client hosts to execute the command.

        Returns:
            bool: whether RPMs have fault-injection.
        """
        result = run_remote(self.log, hosts, "daos_debug_set_params -v 67174515")
        if not result.passed:
            self.fail("Failed to check if fault-injection is enabled")
        for stdout in result.all_stdout.values():
            if not stdout.strip():
                return True
        self.log.info("#Host client rpms did not have fault-injection")
        return False

    def enable_fault_injection(self, hosts):
        """Enable fault injection.

        Args:
            hosts (string, list): hosts to enable fualt injection on.
        """
        if not run_remote(self.log, hosts, "daos_debug_set_params -v 67174515").passed:
            self.fail("Failed to enable fault injection")

    def disable_fault_injection(self, hosts):
        """Disable fault injection.

        Args:
            hosts (string, list): hosts to disable fualt injection on.
        """
        if not run_remote(self.log, hosts, "daos_debug_set_params -v 67108864").passed:
            self.fail("Failed to disable fault injection")

    def verify_pool_upgrade_status(self, pool_id, expected_status):
        """Verify pool upgrade status.

        Args:
            pool_id (str): pool to be verified.
            expected_status (str): pool upgrade expected status.
        """
        prop_value = self.get_dmg_command().pool_get_prop(
            pool_id, "upgrade_status")['response'][0]['value']
        if prop_value != expected_status:
            self.fail("##prop_value != expected_status {}".format(expected_status))

    def pool_upgrade_with_fault(self, hosts, pool_id):
        """Execute dmg pool upgrade with fault injection.

        Args:
            hosts (string, list): client hosts to execute the command.
            pool_id (str): pool to be upgraded
        """
        # Verify pool status before upgrade
        self.verify_pool_upgrade_status(pool_id, expected_status="not started")

        # Enable fault-injection
        self.enable_fault_injection(hosts)

        # Pool upgrade
        if not run_remote(self.log, hosts, "dmg pool upgrade {}".format(pool_id)).passed:
            self.fail("dmg pool upgrade failed")
        # Verify pool status during upgrade
        self.verify_pool_upgrade_status(pool_id, expected_status="in progress")
        # Verify pool status during upgrade
        self.verify_pool_upgrade_status(pool_id, expected_status="failed")

        # Disable fault-injection
        self.disable_fault_injection(hosts)
        # Verify pool upgrade resume after removal of fault-injection
        self.verify_pool_upgrade_status(pool_id, expected_status="completed")

    def diff_versions_agent_server(self):
        """Interoperability of different versions of DAOS agent and server.
        Test step:
            (1) Setup
            (2) dmg system stop
            (3) Upgrade 1 server-host to the new version
            (4) Negative test - dmg pool query on mix-version servers
            (5) Upgrade remaining server hosts to the new version
            (6) Restart old agent
            (7) Verify old agent connects to new server, daos and libdaos
            (8) Upgrade agent to the new version
            (9) Verify pool and containers created with new agent and server
            (10) Downgrade server to the old version
            (11) Verify new agent to old server, daos and libdaos
            (12) Downgrade agent to the old version

        """
        # (1)Setup
        self.log.info("==(1)Setup, create pool and container.")
        hosts_client = self.hostlist_clients
        hosts_server = self.hostlist_servers
        all_hosts = include_local_host(hosts_server | hosts_client)
        self.upgrade_repo = self.params.get("upgrade_repo", '/run/interop/*')
        self.downgrade_repo = self.params.get("downgrade_repo", '/run/interop/*')
        self.old_version = self.params.get("old_version", '/run/interop/*')
        self.new_version = self.params.get("new_version", '/run/interop/*')
        pool = self.get_pool(connect=False)
        pool_id = pool.identifier
        container = self.get_container(pool)
        # container.open()
        cmd = "dmg system query"
        positive_test = True
        negative_test = False
        agent_server_ver = f"{self.old_version} agent to {self.old_version} server"
        self.verify_daos_libdaos("1.1", hosts_client, cmd, positive_test, agent_server_ver)

        self.log_step("Stop all servers and agents")
        self.full_system_stop()

        # (3) Upgrade 1 server-host to new
        self.log.info("==(3)Upgrade 1 server to %s.", self.new_version)
        server = hosts_server[0:1]
        self.upgrade(server, [])
        self.log.info("==(3.1)server %s Upgrade to %s completed.", server, self.new_version)

        # (4) Negative test - dmg pool query on mix-version servers
        self.log.info("==(4)Negative test - dmg pool query on mix-version servers.")
        agent_server_ver = f"{self.old_version} agent to mixed-version servers"
        cmd = "dmg pool list"
        exp_err = "unable to contact the DAOS Management Service"
        self.verify_daos_libdaos(
            "4.1", hosts_client, cmd, negative_test, agent_server_ver, exp_err)

        # (5) Upgrade remaining servers to the new version
        server = hosts_server[1:]
        self.log.info("==(5) Upgrade remaining servers %s to %s.", server, self.new_version)
        self.upgrade(server, [])
        self.log.info("==(5.1) server %s Upgrade to %s completed.", server, self.new_version)

        # (6) Restart old agent
        self.log.info("==(6)Restart %s agent", self.old_version)
        self._start_manager_list("agent", self.agent_managers)
        self.show_daos_version(all_hosts, hosts_client)

        # (7)Verify old agent connect to new server
        self.log.info(
            "==(7)Verify %s agent connect to %s server", self.old_version, self.new_version)
        agent_server_ver = f"{self.old_version} agent to {self.new_version} server"
        cmd = "daos pool query {0}".format(pool_id)
        self.verify_daos_libdaos("7.1", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "dmg pool query {0}".format(pool_id)
        exp_err = "admin:0.0.0 are not compatible"
        self.verify_daos_libdaos(
            "7.2", hosts_client, cmd, negative_test, agent_server_ver, exp_err)
        cmd = "sudo daos_agent dump-attachinfo"
        self.verify_daos_libdaos("7.3", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos cont create {0} --type POSIX --properties 'rf:2'".format(pool_id)
        self.verify_daos_libdaos("7.4", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos pool autotest --pool {0}".format(pool_id)
        self.verify_daos_libdaos("7.5", hosts_client, cmd, positive_test, agent_server_ver)

        # (8) Upgrade agent to the new version
        self.log.info("==(8)Upgrade agent to %s, now %s servers %s agent.",
                      self.new_version, self.new_version, self.new_version)
        self.upgrade([], hosts_client)
        self._start_manager_list("agent", self.agent_managers)
        self.show_daos_version(all_hosts, hosts_client)

        # (9) Pool and containers create on new agent and server
        self.log.info("==(9)Create new pools and containers on %s agent to %s server",
                      self.new_version, self.new_version)
        agent_server_ver = f"{self.new_version} agent to {self.new_version} server"
        cmd = "dmg pool create --size 5G New_pool1"
        self.verify_daos_libdaos("9.1", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "dmg pool list"
        self.verify_daos_libdaos("9.2", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos cont create New_pool1 C21 --type POSIX --properties 'rf:2'"
        self.verify_daos_libdaos("9.3", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos cont create New_pool1 C22 --type POSIX --properties 'rf:2'"
        self.verify_daos_libdaos("9.4", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos container list New_pool1"
        self.verify_daos_libdaos("9.5", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "sudo daos_agent dump-attachinfo"
        self.verify_daos_libdaos("9.6", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos pool autotest --pool New_pool1"
        self.verify_daos_libdaos("9.7", hosts_client, cmd, positive_test, agent_server_ver)

        # (10) Downgrade server to the old version
        self.log.info("==(10) Downgrade server to %s, now %s agent to %s server.",
                      self.old_version, self.new_version, self.old_version)

        self.log_step("Stop all servers and agents")
        self.full_system_stop()

        self.log.info("==(10.2) Downgrade server to %s", self.old_version)
        self.downgrade(hosts_server, [])
        self.log.info("==(10.3) Restart %s agent", self.old_version)
        self._start_manager_list("agent", self.agent_managers)
        self.show_daos_version(all_hosts, hosts_client)

        # (11) Verify new agent to old server
        agent_server_ver = f"{self.new_version} agent to {self.old_version} server"
        cmd = "daos pool query {0}".format(pool_id)
        self.verify_daos_libdaos("11.1", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "dmg pool query {0}".format(pool_id)
        exp_err = "does not match"
        self.verify_daos_libdaos(
            "11.2", hosts_client, cmd, negative_test, agent_server_ver, exp_err)
        cmd = "sudo daos_agent dump-attachinfo"
        self.verify_daos_libdaos("11.3", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos cont create {0} 'C_oldP' --type POSIX --properties 'rf:2'".format(
            pool_id)
        self.verify_daos_libdaos("11.4", hosts_client, cmd, positive_test, agent_server_ver)
        cmd = "daos cont create New_pool1 'C_newP' --type POSIX --properties 'rf:2'"
        exp_err = "DER_NO_SERVICE(-2039)"
        self.verify_daos_libdaos(
            "11.5", hosts_client, cmd, negative_test, agent_server_ver, exp_err)
        exp_err = "common ERR"
        cmd = "daos pool autotest --pool {0}".format(pool_id)
        self.verify_daos_libdaos(
            "11.6", hosts_client, cmd, negative_test, agent_server_ver, exp_err)

        # (12) Downgrade agent to the old version
        self.log.info("==(12)Agent %s  Downgrade started.", hosts_client)
        self.downgrade([], hosts_client)
        self.log.info("==Test passed")

    def upgrade_and_downgrade(self, fault_on_pool_upgrade=False):
        """upgrade and downgrade test base.
        Test step:
            (1)Setup and show rpm, dmg and daos versions on all hosts
            (2)Create pool, container and pool attributes
            (3)Setup and run IOR
                (3.a)DFS
                (3.b)HDF5
                (3.c)POSIX symlink to a file
            (4)Dmg system stop
            (5)Upgrade RPMs to specified new version
            (6)Restart servers
            (7)Restart agent
                verify pool attributes
                verify IOR data integrity after upgraded
                (7.a)DFS
                (7.b)HDF5
                (7.c)POSIX symlink to a file
            (8)Dmg pool get-prop after RPMs upgraded before Pool upgraded
            (9)Dmg pool upgrade and verification after RPMs upgraded
                (9.a)Enable fault injection during pool upgrade
                (9.b)Normal pool upgrade without fault injection
            (10)Create new pool after rpms Upgraded
            (11)Downgrade and cleanup
            (12)Restart servers and agent

        Args:
            fault_on_pool_upgrade (bool): Enable fault-injection during pool upgrade.
        """
        self.log_step("Setup and show rpm, dmg and daos versions on all hosts")
        hosts_client = self.hostlist_clients
        hosts_server = self.hostlist_servers
        all_hosts = include_local_host(hosts_server)
        self.upgrade_repo = self.params.get("upgrade_repo", '/run/interop/*')
        self.downgrade_repo = self.params.get("downgrade_repo", '/run/interop/*')
        self.old_version = self.params.get("old_version", '/run/interop/*')
        self.new_version = self.params.get("new_version", '/run/interop/*')
        num_attributes = self.params.get("num_attributes", '/run/attrtests/*')
        mount_dir = self.params.get("mount_dir", '/run/dfuse/*')
        self.show_daos_version(all_hosts, hosts_client)

        self.log_step("Create pool with attributes")
        self.pool = self.get_pool(connect=False)
        self.daos_cmd = self.get_daos_command()
        pool_attr_dict = self.create_attr_dict(num_attributes)
        self.pool_set_attrs(self.pool.identifier, pool_attr_dict)

        self.log_step("Verify pool attributes")
        self.verify_pool_attrs(self.pool, pool_attr_dict)

        self.log_step("Setup and run IOR")
        self.container = self.get_container(self.pool)
        if not run_remote(self.log, hosts_client, f"mkdir -p {mount_dir}").passed:
            self.fail("Failed to create dfuse mount directory")
        ior_api = self.ior_cmd.api.value
        ior_timeout = self.params.get("ior_timeout", self.ior_cmd.namespace)
        ior_write_flags = self.params.get("write_flags", self.ior_cmd.namespace)
        ior_read_flags = self.params.get("read_flags", self.ior_cmd.namespace)
        testfile = os.path.join(mount_dir, "testfile")
        testfile_sav = os.path.join(mount_dir, "testfile_sav")
        testfile_sav2 = os.path.join(mount_dir, "testfile_sav2")
        symlink_testfile = os.path.join(mount_dir, "symlink_testfile")
        # (3.a) ior dfs
        if ior_api in ("DFS", "POSIX"):
            self.log.info("(3.a)==Run non-HDF5 IOR write and read.")
            self.ior_cmd.update_params(flags=ior_write_flags)
            self.run_ior_with_pool(
                timeout=ior_timeout, create_pool=True, create_cont=True, stop_dfuse=False)
            self.ior_cmd.update_params(flags=ior_read_flags)
            self.run_ior_with_pool(
                timeout=ior_timeout, create_pool=False, create_cont=False, stop_dfuse=False)

        # (3.b)ior hdf5
        elif ior_api == "HDF5":
            self.log.info("(3.b)==Run IOR HDF5 write and read.")
            hdf5_plugin_path = self.params.get("plugin_path", '/run/hdf5_vol/')
            self.ior_cmd.update_params(flags=ior_write_flags)
            self.run_ior_with_pool(
                plugin_path=hdf5_plugin_path, mount_dir=mount_dir,
                timeout=ior_timeout, create_pool=True, create_cont=True, stop_dfuse=False)
            self.ior_cmd.update_params(flags=ior_read_flags)
            self.run_ior_with_pool(
                plugin_path=hdf5_plugin_path, mount_dir=mount_dir,
                timeout=ior_timeout, create_pool=False, create_cont=False, stop_dfuse=False)
        else:
            self.fail("##(3)Unsupported IOR api {}".format(ior_api))

        if ior_api == "POSIX":
            self.log_step("Verify dfuse symlinks")
            cmd_list = [
                f"cd '{mount_dir}'",
                f"ls -l '{testfile}'",
                f"cp '{testfile}' '{testfile_sav}'",
                f"cp '{testfile}' '{testfile_sav2}'",
                f"ln -vs '{testfile_sav2}' '{symlink_testfile}'",
                f"diff '{testfile}' '{testfile_sav}'",
                f"ls -l '{symlink_testfile}'"
            ]
            for cmd in cmd_list:
                if not run_remote(self.log, self.first_client, cmd).passed:
                    self.fail("Failed to verify dfuse symlinks")
            cmd = "fusermount3 -u {}".format(mount_dir)
            if not run_remote(self.log, hosts_client, cmd).passed:
                self.fail("Failed to unmount dfuse")

        self.log_step("Verify pool attributes before upgrade")
        self.verify_pool_attrs(self.pool, pool_attr_dict)

        self.log_step("Stop all servers and agents")
        self.full_system_stop()

        self.log_step(f"Upgrade DAOS to version {self.new_version}")
        self.upgrade(hosts_server, hosts_client)
        self.log.info("==sleeping 30 more seconds")
        time.sleep(30)

        self.log_step(f"Restart v{self.new_version} servers after upgrade")
        self.restart_servers()

        self.log_step(f"Restart v{self.new_version} agents after upgrade")
        self._start_manager_list("agent", self.agent_managers)
        self.show_daos_version(all_hosts, hosts_client)

        self.get_dmg_command().pool_list(verbose=True)
        self.get_dmg_command().pool_query(pool=self.pool.identifier)
        self.daos_cmd.pool_query(pool=self.pool.identifier)

        self.log_step("Verifying pool attributes after upgrade")
        self.verify_pool_attrs(self.pool, pool_attr_dict)
        self.daos_ver_after_upgraded(hosts_client)

        if ior_api == "DFS":
            self.log_step("Verifying read with IOR DFS after upgrade")
            self.run_ior_with_pool(
                timeout=ior_timeout, create_pool=False, create_cont=False, stop_dfuse=False)
        elif ior_api == "HDF5":
            self.log_step("Verifying read with IOR HDF5 after upgrade")
            self.run_ior_with_pool(
                plugin_path=hdf5_plugin_path, mount_dir=mount_dir,
                timeout=ior_timeout, create_pool=False, create_cont=False, stop_dfuse=False)
        else:
            self.log_step("Verifying dfuse symlinks after upgrade")
            cmd = "dfuse --mountpoint {0} --pool {1} --container {2}".format(
                mount_dir, self.pool.identifier, self.container.identifier)
            if not run_remote(self.log, hosts_client, cmd).passed:
                self.fail("Failed to mount dfuse")
            cmd = f"diff {testfile} {testfile_sav}"
            if not run_remote(self.log, hosts_client, cmd).passed:
                self.fail("dfuse files differ after upgrade")
            cmd = f"diff {symlink_testfile} {testfile_sav2}"
            if not run_remote(self.log, hosts_client, cmd).passed:
                self.fail("dfuse files differ after upgrade")

        self.log_step("Call dmg pool get-prop after RPM upgrade, before pool upgrade")
        cmd = f"dmg pool get-prop {self.pool.identifier}"
        if not run_remote(self.log, hosts_client, cmd).passed:
            self.fail("Failed to get pool properties after RPM upgrade")

        if fault_on_pool_upgrade and self.has_fault_injection(hosts_client):
            self.log_step("Call dmg pool upgrade with fault-injection after RPM upgrade")
            self.pool_upgrade_with_fault(hosts_client, self.pool.identifier)
        else:
            self.log_step("Call dmg pool upgrade after RPM upgrade")
            cmd = "dmg pool upgrade {}".format(self.pool.identifier)
            if not run_remote(self.log, hosts_client, cmd).passed:
                self.fail("Failed to upgrade pool")

        self.log_step("Call dmg pool get-prop after RPM upgrade, after pool upgrade")
        cmd = f"dmg pool get-prop {self.pool.identifier}"
        if not run_remote(self.log, hosts_client, cmd).passed:
            self.fail("Failed to get pool properties after pool upgrade")

        self.log_step("Verify pool attributes after dmg pool upgrade")
        self.verify_pool_attrs(self.pool, pool_attr_dict)
        self.pool.destroy(force=True, recursive=False)

        self.log_step("Create a new pool after RPM upgrade")
        self.pool = self.get_pool(connect=False)
        self.get_dmg_command().pool_list(verbose=True)
        self.get_dmg_command().pool_query(pool=self.pool.identifier)
        self.daos_cmd.pool_query(pool=self.pool.identifier)

        self.log_step("Verify dmg pool get-prop on new pool after RPM upgrade")
        cmd = f"dmg pool get-prop {self.pool.identifier}"
        if not run_remote(self.log, hosts_client, cmd).passed:
            self.fail("Failed to get pool properties of new pool after RPM upgrade")

        if ior_api == "POSIX":
            self.log_step("Cleanup dfuse")
            cmd = "fusermount3 -u {}".format(mount_dir)
            if not run_remote(self.log, hosts_client, cmd).passed:
                self.fail("Failed to unmount dfuse")

        self.log_step("Cleanup pool")
        self.pool.destroy(force=True, recursive=False)

        self.log_step("Stop all servers and agents")
        self.full_system_stop()

        self.log_step(f"Downgrade RPMs to {self.old_version}")
        self.downgrade(hosts_server, hosts_client)
        self.log.info("==sleeping 30 more seconds")
        time.sleep(30)

        self.log_step(f"Restart v{self.old_version} servers and agents")
        self.restart_servers()
        self._start_manager_list("agent", self.agent_managers)
        self.show_daos_version(all_hosts, hosts_client)

        if fault_on_pool_upgrade and not self.has_fault_injection(hosts_client):
            self.fail("##(12)Upgraded-rpms did not have fault-injection feature.")

        self.log.info("==(12)Test passed")
