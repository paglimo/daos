/**
 * (C) Copyright 2016-2022 Intel Corporation.
 *
 * SPDX-License-Identifier: BSD-2-Clause-Patent
 */

#include "dfuse_common.h"
#include "dfuse.h"

static inline d_list_t *
dh_hash_find(struct dfuse_projection_info *fs_handle, fuse_ino_t parent, struct dht_call *save)
{
	save->rlink = d_hash_rec_findx(&fs_handle->dpi_iet, &parent, sizeof(parent), NULL,
				       &save->bucket_length, &save->position);

	if (save->rlink) {
		save->promote = false;
		save->dropped = false;

		if (save->position > 10)
			save->promote = true;
		else if (save->bucket_length > 10 && (save->position * 2 > save->bucket_length))
			save->promote = true;
	}
	return save->rlink;
}

void
dfuse_cb_open(fuse_req_t req, fuse_ino_t ino, struct fuse_file_info *fi)
{
	struct dfuse_projection_info *fs_handle = fuse_req_userdata(req);
	struct dfuse_inode_entry     *ie;
	d_list_t		     *rlink;
	struct dfuse_obj_hdl         *oh     = NULL;
	struct fuse_file_info         fi_out = {0};
	struct dht_call               save;
	int                           rc;

	rlink = dh_hash_find(fs_handle, ino, &save);
	if (!rlink) {
		DFUSE_REPLY_ERR_RAW(fs_handle, req, ENOENT);
		return;
	}
	ie = container_of(rlink, struct dfuse_inode_entry, ie_htl);

	D_ALLOC_PTR(oh);
	if (!oh)
		D_GOTO(err, rc = ENOMEM);

	DFUSE_TRA_UP(oh, ie, "open handle");

	dfuse_open_handle_init(oh, ie);

	/* Upgrade fd permissions from O_WRONLY to O_RDWR if wb caching is
	 * enabled so the kernel can do read-modify-write
	 */
	if (ie->ie_dfs->dfc_data_timeout != 0 && fs_handle->dpi_info->di_wb_cache &&
	    (fi->flags & O_ACCMODE) == O_WRONLY) {
		DFUSE_TRA_DEBUG(ie, "Upgrading fd to O_RDRW");
		fi->flags &= ~O_ACCMODE;
		fi->flags |= O_RDWR;
	}

	/** duplicate the file handle for the fuse handle */
	rc = dfs_dup(ie->ie_dfs->dfs_ns, ie->ie_obj, fi->flags, &oh->doh_obj);
	if (rc)
		D_GOTO(err, rc);

	if ((fi->flags & O_ACCMODE) != O_RDONLY)
		oh->doh_writeable = true;

	if (ie->ie_dfs->dfc_data_timeout != 0) {
		if (fi->flags & O_DIRECT)
			fi_out.direct_io = 1;

		if (atomic_load_relaxed(&ie->ie_open_count) > 0) {
			fi_out.keep_cache = 1;
		} else if (dfuse_cache_get_valid(ie, ie->ie_dfs->dfc_data_timeout, NULL)) {
			fi_out.keep_cache = 1;
		}

		if (fi_out.keep_cache)
			oh->doh_keep_cache = true;
	} else {
		fi_out.direct_io = 1;
	}

	if (ie->ie_dfs->dfc_direct_io_disable)
		fi_out.direct_io = 0;

	if (!fi_out.direct_io)
		oh->doh_caching = true;

	fi_out.fh = (uint64_t)oh;

	LOG_FLAGS(ie, fi->flags);

	/*
	 * dfs_dup() just locally duplicates the file handle. If we have
	 * O_TRUNC flag, we need to truncate the file manually.
	 */
	if (fi->flags & O_TRUNC) {
		rc = dfs_punch(ie->ie_dfs->dfs_ns, ie->ie_obj, 0, DFS_MAX_FSIZE);
		if (rc)
			D_GOTO(err, rc);
	}

	atomic_fetch_add_relaxed(&ie->ie_open_count, 1);

	dh_hash_decrefx(fs_handle, &save);
	DFUSE_REPLY_OPEN(oh, req, &fi_out);

	return;
err:
	dh_hash_decrefx(fs_handle, &save);
	D_FREE(oh);
	DFUSE_REPLY_ERR_RAW(ie, req, rc);
}

void
dfuse_cb_release(fuse_req_t req, fuse_ino_t ino, struct fuse_file_info *fi)
{
	struct dfuse_obj_hdl *oh = (struct dfuse_obj_hdl *)fi->fh;
	int                   rc;

	/* Perform the opposite of what the ioctl call does, always change the open handle count
	 * but the inode only tracks number of open handles with non-zero ioctl counts
	 */

	DFUSE_TRA_DEBUG(oh, "Closing %d %d", oh->doh_caching, oh->doh_keep_cache);

	if (atomic_load_relaxed(&oh->doh_write_count) != 0) {
		dfuse_cache_set_time(oh->doh_ie);
		atomic_fetch_sub_relaxed(&oh->doh_ie->ie_open_write_count, 1);
	}

	if (atomic_load_relaxed(&oh->doh_il_calls) != 0) {
		atomic_fetch_sub_relaxed(&oh->doh_ie->ie_il_count, 1);
	}
	if (oh->doh_caching && !oh->doh_keep_cache)
		dfuse_cache_set_time(oh->doh_ie);
	atomic_fetch_sub_relaxed(&oh->doh_ie->ie_open_count, 1);

	rc = dfs_release(oh->doh_obj);
	if (rc == 0)
		DFUSE_REPLY_ZERO(oh, req);
	else
		DFUSE_REPLY_ERR_RAW(oh, req, rc);
	D_FREE(oh);
}
