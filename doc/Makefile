#
# Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
#
# SPDX-License-Identifier: BSD-2-Clause
#

AGENT_STATUS_CBOR := example-agent-status.cbor
AGENT_STATUS_CDDL := concatenated-agent-status.cddl

CDDL := cddl
CDDLC := cddlc
CBOR2DIAG := cbor2diag.rb
SUIT_MANIFEST_CDDL := draft-ietf-suit-manifest.cddl
RFC9711_CDDL := rfc9711.cddl

.PHONY: all
all: $(AGENT_STATUS_CBOR)

%.cbor: %.diag
	$(CBOR2DIAG) < $< > $@

$(AGENT_STATUS_CDDL): agent-status.cddl $(SUIT_MANIFEST_CDDL) $(RFC9711_CDDL)
	$(CDDLC) -u -2 -t cddl -i $(basename $(RFC9711_CDDL)) $< $(SUIT_MANIFEST_CDDL) > $@

$(SUIT_MANIFEST_CDDL):
	curl https://raw.githubusercontent.com/suit-wg/manifest-spec/refs/heads/master/draft-ietf-suit-manifest.cddl > $@

$(RFC9711_CDDL):
	curl https://raw.githubusercontent.com/ietf-rats-wg/draft-ietf-rats-ear/refs/heads/main/cddl/rfc9711.cddl > $@

.PHONY: test
test: $(AGENT_STATUS_CBOR) $(AGENT_STATUS_CDDL)
	$(CDDL) $(AGENT_STATUS_CDDL) validate $(AGENT_STATUS_CBOR)

.PHONY: clean
clean:
	$(RM) $(AGENT_STATUS_CBOR) $(AGENT_STATUS_CDDL)
