#
# Copyright (c) 2025 SECOM CO., LTD. All Rights reserved.
#
# SPDX-License-Identifier: BSD-2-Clause
#

AGENT_STATUS_CBOR := example-agent-status.cbor
AGENT_STATUS_CDDL := concatenated-agent-status.cddl
SUIT_MANIFEST_OVERVIEW_CBOR := example-suit-manifest-overview.cbor
SUIT_MANIFEST_OVERVIEW_CDDL := concatenated-suit-manifest-overview.cddl

RUBYDEBUG=RUBYOPT="-W0"
CDDL := $(RUBYDEBUG) cddl
CDDLC := cddlc
DIAG2CBOR := diag2cbor.rb
SUIT_MANIFEST_CDDL := draft-ietf-suit-manifest.cddl
RFC9711_CDDL := rfc9711.cddl

.PHONY: all
all: $(AGENT_STATUS_CBOR) $(SUIT_MANIFEST_OVERVIEW_CBOR)

%.cbor: %.diag
	$(DIAG2CBOR) < $< > $@

$(AGENT_STATUS_CDDL): agent-status.cddl suit-manifest-overview.cddl $(SUIT_MANIFEST_CDDL) $(RFC9711_CDDL)
	$(CDDLC) -u -2 -t cddl -i $(basename $(RFC9711_CDDL)) agent-status.cddl suit-manifest-overview.cddl $(SUIT_MANIFEST_CDDL) > $@

$(SUIT_MANIFEST_OVERVIEW_CDDL): suit-manifest-overview.cddl $(SUIT_MANIFEST_CDDL)
	$(CDDLC) -u -2 -t cddl $^ > $@

$(SUIT_MANIFEST_CDDL):
	curl https://raw.githubusercontent.com/suit-wg/manifest-spec/refs/heads/master/draft-ietf-suit-manifest.cddl > $@

$(RFC9711_CDDL):
	curl https://raw.githubusercontent.com/ietf-rats-wg/draft-ietf-rats-ear/refs/heads/main/cddl/rfc9711.cddl > $@

.PHONY: test-agent-status
test-agent-status: $(AGENT_STATUS_CBOR) $(AGENT_STATUS_CDDL)
	$(CDDL) $(AGENT_STATUS_CDDL) validate $(AGENT_STATUS_CBOR)
	@echo [PASS] getAgent example matches to the CDDL

.PHONY: test-suit-manifest-overview
test-suit-manifest-overview: $(SUIT_MANIFEST_OVERVIEW_CBOR) $(SUIT_MANIFEST_OVERVIEW_CDDL)
	$(CDDL) $(SUIT_MANIFEST_OVERVIEW_CDDL) validate $(SUIT_MANIFEST_OVERVIEW_CBOR)
	@echo [PASS] getManifests example matches to the CDDL

.PHONY: test
test: test-agent-status test-suit-manifest-overview

.PHONY: clean
clean:
	$(RM) $(AGENT_STATUS_CBOR) $(AGENT_STATUS_CDDL) $(SUIT_MANIFEST_OVERVIEW_CBOR) $(SUIT_MANIFEST_OVERVIEW_CDDL) $(RFC9711_CDDL) $(SUIT_MANIFEST_CDDL)
