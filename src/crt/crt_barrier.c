/* Copyright (C) 2017 Intel Corporation
 * All rights reserved.
 *
 * Redistribution and use in source and binary forms, with or without
 * modification, are permitted for any purpose (including commercial purposes)
 * provided that the following conditions are met:
 *
 * 1. Redistributions of source code must retain the above copyright notice,
 *    this list of conditions, and the following disclaimer.
 *
 * 2. Redistributions in binary form must reproduce the above copyright notice,
 *    this list of conditions, and the following disclaimer in the
 *    documentation and/or materials provided with the distribution.
 *
 * 3. In addition, redistributions of modified forms of the source or binary
 *    code must carry prominent notices stating that the original code was
 *    changed and the date of the change.
 *
 *  4. All publications or advertising materials mentioning features or use of
 *     this software are asked, but not required, to acknowledge that it was
 *     developed by Intel Corporation and credit the contributors.
 *
 * 5. Neither the name of Intel Corporation, nor the name of any Contributor
 *    may be used to endorse or promote products derived from this software
 *    without specific prior written permission.
 *
 * THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS"
 * AND ANY EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE
 * IMPLIED WARRANTIES OF MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE
 * ARE DISCLAIMED. IN NO EVENT SHALL THE COPYRIGHT HOLDER BE LIABLE FOR ANY
 * DIRECT, INDIRECT, INCIDENTAL, SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES
 * (INCLUDING, BUT NOT LIMITED TO, PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES;
 * LOSS OF USE, DATA, OR PROFITS; OR BUSINESS INTERRUPTION) HOWEVER CAUSED AND
 * ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT, STRICT LIABILITY, OR TORT
 * (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF THE USE OF
 * THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.
 */
/**
 * This file is part of CaRT. It implements the barrier APIs.
 */

#include <crt_internal.h>

void
crt_barrier_info_init(struct crt_grp_priv *grp_priv)
{
	struct crt_barrier_info	*info;
	crt_group_t		*grp;

	info = &grp_priv->gp_barrier_info;

	pthread_mutex_init(&info->bi_lock, NULL);

	/* Default barrier master is the lowest numbered rank.  At startup,
	 * it's index 0.  It gets updated in crt_barrier_update_master
	 */
	if (grp_priv->gp_membs == NULL)
		info->bi_master_pri_rank = 0;
	else
		info->bi_master_pri_rank = grp_priv->gp_membs->rl_ranks[0];
	info->bi_master_idx = 0;
	if (grp_priv->gp_primary)
		info->bi_primary_grp = grp_priv;
	else {
		/* Get the primary group */
		grp = crt_group_lookup(NULL);
		/* We can only get here if crt has been initialized so this
		 * should not fail
		 */
		C_ASSERTF(grp != NULL,
			  "crt_barrier: Primary group lookup failed");
		info->bi_primary_grp = container_of(grp, struct crt_grp_priv,
						    gp_pub);
	}

	/* Eventually, this will be handled by a flag passed to the corpc
	 * routine but until then, create a list to exclude self from broadcast
	 */
	info->bi_exclude_self = crt_rank_list_alloc(1);
	C_ASSERTF(info->bi_exclude_self != NULL,
		  "No memory to allocate barrier");

	info->bi_exclude_self->rl_ranks[0] = info->bi_primary_grp->gp_self;
	info->bi_exclude_self->rl_nr.num = 1;
}

void
crt_barrier_info_destroy(struct crt_grp_priv *grp_priv)
{
	pthread_mutex_destroy(&grp_priv->gp_barrier_info.bi_lock);
	crt_rank_list_free(grp_priv->gp_barrier_info.bi_exclude_self);
}

/* Update the master rank.  Returns true if the master has changed since
 * the last update.
 */
bool
crt_barrier_update_master(struct crt_grp_priv *grp_priv)
{
	struct crt_barrier_info		*info;
	struct crt_grp_priv		*primary_grp;
	crt_rank_t			 rank;
	bool				 new_master = false;
	int				 i;

	info = &grp_priv->gp_barrier_info;

	primary_grp = info->bi_primary_grp;

	pthread_mutex_lock(&info->bi_lock);

	pthread_rwlock_rdlock(&primary_grp->gp_rwlock);
	if (crt_rank_in_rank_list(grp_priv->gp_failed_ranks,
				  info->bi_master_pri_rank, true)) {
		rank = -1;
		/* Master has failed */
		new_master = true;
		for (i = info->bi_master_idx + 1;
		     i < grp_priv->gp_membs->rl_nr.num; i++) {
			rank = grp_priv->gp_membs->rl_ranks[i];
			if (!crt_rank_in_rank_list(grp_priv->gp_failed_ranks,
						   rank, true))
				break;
		}

		/* This should be impossible since the current rank, at least,
		 * is still alive and can be the master.
		 */
		C_ASSERTF(i != grp_priv->gp_membs->rl_nr.num,
			  "No more ranks for barrier");
		C_ASSERTF(rank != -1, "No more ranks for barrier");
		info->bi_master_pri_rank = rank;
		info->bi_master_idx = i;
	}

	pthread_rwlock_unlock(&primary_grp->gp_rwlock);
	pthread_mutex_unlock(&info->bi_lock);

	return new_master;
}

/* Callback for enter broadcast
 * All non-master ranks execute this callback
 */
int
crt_hdlr_barrier_enter(crt_rpc_t *rpc_req)
{
	struct crt_barrier_in		*in;
	struct crt_barrier_out		*out;
	struct crt_barrier		*ab;
	struct crt_barrier_info		*barrier_info = NULL;
	struct crt_grp_priv		*grp_priv;
	int				rc = 0;

	in = crt_req_get(rpc_req);
	C_ASSERT(in != NULL);

	if (rpc_req->cr_ep.ep_grp == NULL)
		grp_priv = crt_gdata.cg_grp->gg_srv_pri_grp;
	else
		grp_priv = container_of(rpc_req->cr_ep.ep_grp,
					struct crt_grp_priv, gp_pub);

	if (grp_priv == NULL) {
		C_ERROR("crt_hdlr_barrier_enter failed, no group\n");
		C_GOTO(send_reply, rc = -CER_NONEXIST);
	}

	barrier_info = &grp_priv->gp_barrier_info;

	pthread_mutex_lock(&barrier_info->bi_lock);

	C_DEBUG("barrier enter msg received for %d\n", in->b_num);

	if (barrier_info->bi_num_exited >= in->b_num) {
		/* It's a duplicate.   Send the reply again */
		C_GOTO(send_reply, rc = 0);
	}

	ab = &barrier_info->bi_barriers[in->b_num % CRT_MAX_BARRIER_INFLIGHT];

	if (!ab->b_active) {
		/* Local node hasn't arrived yet */
		ab->b_enter_rpc = rpc_req;
		/* decref in crt_barrier */
		crt_req_addref(rpc_req);
		pthread_mutex_unlock(&barrier_info->bi_lock);
		return 0;
	}

	/* Local node already arrived.   Send a reply.  This could happen
	 * more than once in presence of node failures but it doesn't matter.
	 */
send_reply:

	pthread_mutex_unlock(&barrier_info->bi_lock);
	out = crt_reply_get(rpc_req);
	C_ASSERT(out != NULL);

	out->b_rc = rc;

	rc = crt_reply_send(rpc_req);

	/* If the reply is lost, timeout will try again */
	if (rc != 0)
		C_ERROR("Could not send reply for barrier broadcast,rc = %d\n",
			rc);

	return rc;
}

/* Callback for exit broadcast signalling that all ranks have arrived
 * All non-master ranks execute this callback
 */
int
crt_hdlr_barrier_exit(crt_rpc_t *rpc_req)
{
	crt_barrier_cb_t		complete_cb = NULL;
	struct crt_barrier_cb_info	cb_info;
	struct crt_barrier_in		*in;
	struct crt_barrier_out		*out;
	struct crt_barrier		*ab = NULL;
	struct crt_barrier_info		*barrier_info = NULL;
	struct crt_grp_priv		*grp_priv;
	int				rc = 0;

	in = crt_req_get(rpc_req);
	out = crt_reply_get(rpc_req);
	C_ASSERT(in != NULL && out != NULL);

	if (rpc_req->cr_ep.ep_grp == NULL)
		grp_priv = crt_gdata.cg_grp->gg_srv_pri_grp;
	else
		grp_priv = container_of(rpc_req->cr_ep.ep_grp,
					struct crt_grp_priv, gp_pub);

	if (grp_priv == NULL) {
		C_ERROR("crt_barrier_enter failed, no group\n");
		C_GOTO(send_reply, rc = -CER_NONEXIST);
	}

	barrier_info = &grp_priv->gp_barrier_info;
	C_DEBUG("barrier exit msg received for %d\n", in->b_num);

	pthread_mutex_lock(&barrier_info->bi_lock);

	if (barrier_info->bi_num_exited >= in->b_num) {
		/* Duplicate message.  Send reply again */
		C_DEBUG("barrier exit msg %d is duplcate\n", in->b_num);
		C_GOTO(send_reply, rc = 0);
	}

	C_ASSERTF(in->b_num == (barrier_info->bi_num_exited + 1),
		  "Barrier exit out of order\n");

	barrier_info->bi_num_exited = in->b_num;
	ab = &barrier_info->bi_barriers[in->b_num % CRT_MAX_BARRIER_INFLIGHT];
	ab->b_active = false;
	complete_cb = ab->b_complete_cb;
	cb_info.bci_rc = 0;
	cb_info.bci_arg = ab->b_arg;
	complete_cb = ab->b_complete_cb;
	ab->b_complete_cb = NULL;
	ab->b_arg = NULL;
send_reply:
	pthread_mutex_unlock(&barrier_info->bi_lock);

	if (complete_cb != NULL) {
		/* Execute completion callback */
		complete_cb(&cb_info);
	}

	out->b_rc = rc;
	rc = crt_reply_send(rpc_req);

	/* If the reply is lost, timeout will try again */
	if (rc != 0)
		C_ERROR("Could not send reply for barrier broadcast,rc = %d\n",
			rc);

	return rc;
}

int
crt_hdlr_barrier_aggregate(crt_rpc_t *source, crt_rpc_t *result,
			       void *priv)
{
	int	*reply_source, *reply_result;

	C_ASSERT(source != NULL && result != NULL);
	reply_source = crt_reply_get(source);
	reply_result = crt_reply_get(result);
	C_ASSERT(reply_source != NULL && reply_result != NULL);
	if (*reply_result == 0)
		*reply_result = *reply_source;

	return 0;
}

/* The barrier master sends broadcast messages to other ranks signaling
 * start or completion of the barrier.  It is assumes the following about
 * broadcast:
 *
 * 1. Group membership changes will be handled internally and completion
 * callback is only invoked when all current members have received the
 * message.
 * 2. Failed ranks are automatically excluded
 *
 * Neither condition is true today.
 */
static void
send_barrier_msg(struct crt_grp_priv *grp_priv, int b_num,
		 crt_cb_t complete_cb, int opcode)
{
	crt_rpc_t			*rpc_req;
	crt_barrier_cb_t		b_complete = NULL;
	struct crt_barrier_cb_info	cb_info;
	struct crt_barrier		*ab;
	struct crt_barrier_info		*barrier_info;
	struct crt_barrier_in		*in;
	crt_context_t			crt_ctx;
	int				rc;

	/* Context 0 is required and this condition is checked in
	 * crt_barrier_create so assertion is fine.
	 */
	crt_ctx = crt_context_lookup(0);
	C_ASSERT(crt_ctx != CRT_CONTEXT_NULL);
	C_DEBUG("Sending barrier message for %d\n", b_num);

	/* TODO: Eventually, there will be a flag to exclude self from
	 * from the broadcast.  Until then, the rank list including
	 * self will suffice.
	 */
	rc = crt_corpc_req_create(crt_ctx, &grp_priv->gp_pub,
			     grp_priv->gp_barrier_info.bi_exclude_self,
			     opcode, NULL, NULL, 0,
			     crt_tree_topo(CRT_TREE_KNOMIAL, 4), &rpc_req);

	/* If this fails, we have nothing to do but fail the barrier
	 * and let the user deal with it
	 */
	if (rc != 0) {
		C_ERROR("Failed to create barrier opc %d rpc, rc = %d",
			opcode, rc);
		C_GOTO(handle_error, rc);
	}
	C_DEBUG("Created req for %d\n", b_num);
	in = crt_req_get(rpc_req);

	in->b_num = b_num;

	rc = crt_req_send(rpc_req, complete_cb, NULL);

	C_DEBUG("Sent req for %d\n", b_num);
	if (rc != 0) {
		C_ERROR("Failed to send barrier opc %d rpc, rc = %d",
			opcode, rc);
		C_GOTO(handle_error, rc);
	}
	return;
handle_error:
	barrier_info = &grp_priv->gp_barrier_info;
	C_ERROR("Critical failure in barrier master, rc = %d\n", rc);
	/* Assume all errors in this function are unrecoverable */
	pthread_mutex_lock(&barrier_info->bi_lock);
	ab = &barrier_info->bi_barriers[b_num % CRT_MAX_BARRIER_INFLIGHT];
	ab->b_active = false;
	cb_info.bci_rc = rc;
	cb_info.bci_arg = ab->b_arg;
	b_complete = ab->b_complete_cb;
	ab->b_complete_cb = NULL;
	ab->b_arg = NULL;
	pthread_mutex_unlock(&barrier_info->bi_lock);

	if (b_complete != NULL)
		b_complete(&cb_info);
}

static int
barrier_exit_cb(const struct crt_cb_info *cb_info)
{
	struct crt_grp_priv		*grp_priv;
	struct crt_barrier		*ab;
	struct crt_barrier_info		*barrier_info;
	struct crt_barrier_in		*in;
	struct crt_barrier_out		*out;
	crt_barrier_cb_t		complete_cb = NULL;
	struct crt_barrier_cb_info	info;
	crt_rpc_t			*rpc_req;
	int				b_num;

	rpc_req = cb_info->cci_rpc;

	out = crt_reply_get(rpc_req);
	in = crt_req_get(rpc_req);

	if (rpc_req->cr_ep.ep_grp == NULL)
		grp_priv = crt_gdata.cg_grp->gg_srv_pri_grp;
	else
		grp_priv = container_of(rpc_req->cr_ep.ep_grp,
					struct crt_grp_priv, gp_pub);
	C_ASSERT(grp_priv != NULL);

	if (cb_info->cci_rc != 0 || out->b_rc != 0) {
		/* Resend the exit message */
		send_barrier_msg(grp_priv, in->b_num, barrier_exit_cb,
				 CRT_OPC_BARRIER_EXIT);
		return 0;
	}
	C_DEBUG("Exit phase complete for %d\n", in->b_num);

	barrier_info = &grp_priv->gp_barrier_info;
	ab = &barrier_info->bi_barriers[in->b_num % CRT_MAX_BARRIER_INFLIGHT];

	pthread_mutex_lock(&barrier_info->bi_lock);
	C_ASSERTF(barrier_info->bi_num_exited == (in->b_num - 1),
		  "Barrier exit out of order");

	if (barrier_info->bi_num_exited < in->b_num) {
		/* otherwise, this is a replay */
		barrier_info->bi_num_exited = in->b_num;
		ab->b_active = false;
		info.bci_rc = 0;
		info.bci_arg = ab->b_arg;
		complete_cb = ab->b_complete_cb;
		ab->b_complete_cb = NULL;
		ab->b_arg = NULL;
	}
	pthread_mutex_unlock(&barrier_info->bi_lock);

	if (complete_cb != NULL)
		complete_cb(&info);

	/* Ok, now check if the next barrier is pending_exit */
	b_num = in->b_num + 1;
	pthread_mutex_lock(&barrier_info->bi_lock);
	ab = &barrier_info->bi_barriers[b_num % CRT_MAX_BARRIER_INFLIGHT];
	if (!ab->b_active || !ab->b_pending_exit)
		ab = NULL;
	else
		ab->b_pending_exit = false;
	pthread_mutex_unlock(&barrier_info->bi_lock);

	if (ab != NULL) {
		/* Send exit message for next barrier */
		send_barrier_msg(grp_priv, b_num, barrier_exit_cb,
				 CRT_OPC_BARRIER_EXIT);
	}

	return 0;
}

static int
barrier_enter_cb(const struct crt_cb_info *cb_info)
{
	struct crt_grp_priv	*grp_priv;
	struct crt_barrier	*ab;
	struct crt_barrier_info	*barrier_info;
	crt_rpc_t		*rpc_req;
	struct crt_barrier_out	*out;
	struct crt_barrier_in	*in;
	bool			send_exit = false;

	rpc_req = cb_info->cci_rpc;

	out = crt_reply_get(rpc_req);
	in = crt_req_get(rpc_req);

	if (rpc_req->cr_ep.ep_grp == NULL)
		grp_priv = crt_gdata.cg_grp->gg_srv_pri_grp;
	else
		grp_priv = container_of(rpc_req->cr_ep.ep_grp,
					struct crt_grp_priv, gp_pub);

	C_ASSERT(grp_priv != NULL);

	if (cb_info->cci_rc != 0 || out->b_rc != 0) {
		/* Resend the enter message */
		send_barrier_msg(grp_priv, in->b_num, barrier_enter_cb,
				 CRT_OPC_BARRIER_ENTER);
		return 0;
	}

	C_DEBUG("Enter phase complete for %d\n", in->b_num);

	barrier_info = &grp_priv->gp_barrier_info;
	ab = &barrier_info->bi_barriers[in->b_num % CRT_MAX_BARRIER_INFLIGHT];
	pthread_mutex_lock(&barrier_info->bi_lock);

	ab->b_pending_exit = true;

	/* If we've processed prior exits, we can go ahead and send the exit */
	if (barrier_info->bi_num_exited == (in->b_num - 1)) {
		send_exit = true;
		ab->b_pending_exit = false;
	}

	pthread_mutex_unlock(&barrier_info->bi_lock);

	if (send_exit)
		send_barrier_msg(grp_priv, in->b_num, barrier_exit_cb,
				 CRT_OPC_BARRIER_EXIT);

	return 0;
}

int
crt_barrier(crt_group_t *grp, crt_barrier_cb_t complete_cb, void *cb_arg)
{
	struct crt_context_t		*crt_ctx;
	struct crt_barrier_info		*barrier_info;
	struct crt_barrier		*ab;
	struct crt_grp_priv		*grp_priv;
	crt_rpc_t			*rpc_req = NULL;
	struct crt_barrier_out		*out;
	struct crt_barrier_cb_info	info;
	int				enter_num;

	if (!crt_initialized()) {
		C_ERROR("CRT not initialized.\n");
		return -CER_UNINIT;
	}

	if (!crt_is_service()) {
		C_ERROR("Barrier not supported in client group\n");
		return -CER_NO_PERM;
	}

	crt_ctx = crt_context_lookup(0);
	if (crt_ctx == CRT_CONTEXT_NULL) {
		C_ERROR("No context available for barrier\n");
		return -CER_UNINIT;
	}

	if (complete_cb == NULL) {
		C_ERROR("Invalid argument(s)\n");
		return -CER_INVAL;
	}

	/* There may be a better way to get the primary group handle but this
	 * does the trick for now.
	 */
	if (grp == NULL)
		grp = crt_group_lookup(NULL);

	if (grp == NULL) {
		C_ERROR("Could not find primary group\n");
		return -CER_UNINIT;
	}

	grp_priv = container_of(grp, struct crt_grp_priv, gp_pub);

	if (grp_priv->gp_primary != 1) {
		C_ERROR("Barrier not supported on secondary groups.\n");
		return -CER_OOG;
	}

	if (grp_priv->gp_local == 0) {
		C_ERROR("Barrier not supported on remote group.\n");
		return -CER_OOG;
	}

	if (grp_priv->gp_size == 1) {
		/* No need for broadcast */
		info.bci_rc = 0;
		info.bci_arg = cb_arg;

		complete_cb(&info);
		return 0;
	}

	barrier_info = &grp_priv->gp_barrier_info;

	pthread_mutex_lock(&barrier_info->bi_lock);
	enter_num = barrier_info->bi_num_created + 1;

	ab = &barrier_info->bi_barriers[enter_num % CRT_MAX_BARRIER_INFLIGHT];

	if (ab->b_active) {
		pthread_mutex_unlock(&barrier_info->bi_lock);
		return -CER_BUSY;
	}

	ab->b_active = true;

	ab->b_arg = cb_arg;
	ab->b_complete_cb = complete_cb;
	/* If master already arrived, this field will be non-NULL.  We save
	 * the value so we can reply
	 */
	rpc_req = ab->b_enter_rpc;
	ab->b_enter_rpc = NULL;

	barrier_info->bi_num_created = enter_num;

	pthread_mutex_unlock(&barrier_info->bi_lock);

	if (rpc_req != NULL) {
		out = crt_reply_get(rpc_req);

		out->b_rc = 0;
		crt_reply_send(rpc_req);

		/* addref in crt_hdlr_barrier_enter_cb */
		crt_req_decref(rpc_req);
	}

	if (barrier_info->bi_master_pri_rank == grp_priv->gp_self)
		send_barrier_msg(grp_priv, enter_num, barrier_enter_cb,
				 CRT_OPC_BARRIER_ENTER);

	C_DEBUG("barrier %d started\n", enter_num);

	return 0;
}

void
crt_barrier_handle_eviction(struct crt_grp_priv *grp_priv)
{
	bool			updated;
	struct crt_barrier_info	*barrier_info;
	int			saved_exited;
	int			saved_created;

	/* We only handle barriers for primary group at present but this is
	 * the code that would need to change to cycle through more than
	 * just the primary group.
	 */

	updated = crt_barrier_update_master(grp_priv);

	if (!updated) /* Same master as before */
		return;

	barrier_info = &grp_priv->gp_barrier_info;

	if (barrier_info->bi_master_pri_rank != grp_priv->gp_self)
		return; /* New master is another rank */

	/* Ok, we are the new master.   We need to replay the last enter
	 * message and exit messages received.
	 */
	pthread_mutex_lock(&barrier_info->bi_lock);
	saved_exited = barrier_info->bi_num_exited;
	saved_created = barrier_info->bi_num_created;
	pthread_mutex_unlock(&barrier_info->bi_lock);

	/* First send the exit message remote ranks may have missed */
	C_DEBUG("New master sending exit for %d\n", saved_exited);
	send_barrier_msg(grp_priv, saved_exited,  barrier_exit_cb,
			 CRT_OPC_BARRIER_EXIT);
	/* Now send any enter messages that remote nodes may have missed */
	saved_exited++;
	for (; saved_exited <= saved_created; ++saved_exited) {
		C_DEBUG("New master sending enter for %d\n", saved_exited);
		send_barrier_msg(grp_priv, saved_exited,  barrier_enter_cb,
				 CRT_OPC_BARRIER_ENTER);
	}
	send_barrier_msg(grp_priv, saved_exited,  barrier_exit_cb,
			 CRT_OPC_BARRIER_EXIT);
}
