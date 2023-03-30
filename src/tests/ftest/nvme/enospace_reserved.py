'''
  (C) Copyright 2020-2023 Intel Corporation.

  SPDX-License-Identifier: BSD-2-Clause-Patent
'''
from nvme_utils import ServerFillUp


class NvmeEnospaceReserved(ServerFillUp):
    """Tests pool DER_NOSPACE errors with reserved space.
    :avocado: recursive
    """

    def test_nvme_enospace_reserved(self):
        """Jira ID: DAOS-12958.

        Test Description:
            1. Create a pool with DAOS_PROP_SPACE_RB set and aggregation disabled.
            2. Using 2K IO size, fill SCM to (100 - DAOS_PROP_SPACE_RB) percent.
            3. Verify IO fails with DER_NOSPACE.
            2. Using 8K IO size, fill NVMe to (100 - DAOS_PROP_SPACE_RB) percent.
            3. Verify IO fails with DER_NOSPACE.

        :avocado: tags=all,full_regression
        :avocado: tags=hw,medium
        :avocado: tags=nvme,der_enospace
        :avocado: tags=NvmeEnospaceReserved,test_nvme_enospace_reserved
        """
        self.pool = self.get_pool()

        # TODO complete and test
        self.start_ior_load(storage='SCM', operation="Auto_Write", percent=50)
        # self.start_ior_load(storage='NVMe', operation="Auto_Write", percent=50)
