// Copyright 2022 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.
import {
  JobDetails,
  JobDetailsStateProps,
  selectID,
} from "@cockroachlabs/cluster-ui";
import { connect } from "react-redux";
import { RouteComponentProps, withRouter } from "react-router-dom";
import {
  createSelectorForKeyedCachedDataField,
  refreshListExecutionDetailFiles,
  refreshJob,
  refreshUserSQLRoles,
} from "src/redux/apiReducers";
import { AdminUIState, AppDispatch } from "src/redux/state";
import { ListJobProfilerExecutionDetailsResponseMessage } from "src/util/api";
import { api as clusterUiApi } from "@cockroachlabs/cluster-ui";
import { collectExecutionDetailsAction } from "oss/src/redux/jobs/jobsActions";
import long from "long";
import { selectHasAdminRole } from "src/redux/user";

const selectJob = createSelectorForKeyedCachedDataField("job", selectID);
const selectExecutionDetailFiles =
  createSelectorForKeyedCachedDataField<ListJobProfilerExecutionDetailsResponseMessage>(
    "jobProfiler",
    selectID,
  );
const mapStateToProps = (
  state: AdminUIState,
  props: RouteComponentProps,
): JobDetailsStateProps => {
  const jobID = selectID(state, props);
  return {
    jobRequest: selectJob(state, props),
    jobProfilerExecutionDetailFilesResponse: selectExecutionDetailFiles(
      state,
      props,
    ),
    jobProfilerLastUpdated: state.cachedData.jobProfiler[jobID]?.setAt,
    jobProfilerDataIsValid: state.cachedData.jobProfiler[jobID]?.valid,
    onDownloadExecutionFileClicked: clusterUiApi.getExecutionDetailFile,
    hasAdminRole: selectHasAdminRole(state),
  };
};

const mapDispatchToProps = {
  refreshJob,
  refreshExecutionDetailFiles: refreshListExecutionDetailFiles,
  onRequestExecutionDetails: (jobID: long) => {
    return (dispatch: AppDispatch) => {
      dispatch(collectExecutionDetailsAction({ job_id: jobID }));
    };
  },
  refreshUserSQLRoles: refreshUserSQLRoles,
};

export default withRouter(
  connect(mapStateToProps, mapDispatchToProps)(JobDetails),
);
