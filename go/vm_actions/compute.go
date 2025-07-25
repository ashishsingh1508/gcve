package vm_actions

import (
	"context"
	"fmt"
	log "gcveadmin/gcvelogger"
	aria "gcveadmin/operations/deployments/vmwarearia"

	"github.com/vmware/govmomi"
	"github.com/vmware/govmomi/vim25/methods"
	"github.com/vmware/govmomi/vim25/mo"
	"github.com/vmware/govmomi/vim25/types"
)

type Compute struct {
	MemoryInMB      int64
	ComputeCPU      int32
	DiskSizeInBytes int64
}

type ComputeResizer interface {
	ResizeCompute(context.Context, Compute, string)
}

type Client struct {
	vcClient *govmomi.Client
}

// VMClient holds VM details with
// VC Client
type VMClient struct {
	Client
	VM mo.VirtualMachine
	Compute
}

// Modified: config.hardware.device(2000).deviceInfo.summary: "11,075,584 KB" -> "11,534,336 KB"; config.hardware.device(2000).backing.contentId: "d462f979da7f2a239d0b5e90e8811482" -> "c97fca3436fa2c4e0e4de9e407ce6840";
// config.hardware.device(2000).capacityInKB: 11075584 -> 11534336; config.hardware.device(2000).capacityInBytes: 11341398016 -> 11811160064; config.cpuAllocation.shares.shares: 2000 -> 32000; config.memoryAllocation.shares.shares: 61440 -> 491520;

func (vm VMClient) ResizeComputeAndHardDisk(ctx context.Context, computeData Compute, hdLabelToExtend string) error {
	vmRef, err := aria.VMDetailsByName(ctx, vm.vcClient, vm.VM.Name)
	if err != nil || len(vmRef) == 0 {
		log.WriteErrorLog(fmt.Sprintf("failed to get VM details. ref: %v", vmRef))
		return err
	}
	var hd types.BaseVirtualDevice
	for _, eachVMDevice := range vmRef[0].Config.Hardware.Device {
		log.WriteInfoLog(fmt.Sprintf("device details: %+v", eachVMDevice.GetVirtualDevice().DeviceInfo))
		if eachVMDevice.GetVirtualDevice().DeviceInfo.GetDescription().Label == hdLabelToExtend {
			hd = eachVMDevice
			break
		}
	}
	vd, ok := hd.(*types.VirtualDisk)
	if !ok {
		return fmt.Errorf("unable to assert the hard disk value")
	}
	log.WriteInfoLog(fmt.Sprintf("One disk info %+v, \nKey %+v", hd, hd.GetVirtualDevice()))
	req := &types.ReconfigVM_Task{
		This: vmRef[0].Self,
		Spec: types.VirtualMachineConfigSpec{
			NumCPUs:  computeData.ComputeCPU,
			MemoryMB: computeData.MemoryInMB,
			DeviceChange: []types.BaseVirtualDeviceConfigSpec{
				&types.VirtualDeviceConfigSpec{
					Operation: types.VirtualDeviceConfigSpecOperationEdit,
					Device:    vd,
				},
			},
		},
	}
	vd.CapacityInBytes = computeData.DiskSizeInBytes
	resp, err := methods.ReconfigVM_Task(ctx, vm.vcClient, req)
	if err != nil {
		return err
	}
	taskInfo := aria.WaitForTaskResult(ctx, vm.vcClient, resp.Returnval)
	fmt.Println(taskInfo.Result)
	return nil
}

// ReserveCPUAndMemory reserves given cpu and memory fully
// Shares of reservation is set to normal and currently changing is not
// supported. No NUMA node is configured either.
func ResizeCompute(ctx context.Context, vc *govmomi.Client, computeData Compute, vmRef mo.VirtualMachine) error {
	req := &types.ReconfigVM_Task{
		This: vmRef.Self,
		Spec: types.VirtualMachineConfigSpec{
			NumCPUs:  computeData.ComputeCPU,
			MemoryMB: computeData.MemoryInMB,
		},
	}
	// vd.CapacityInBytes = computeData.DiskSizeInBytes
	resp, err := methods.ReconfigVM_Task(ctx, vc, req)
	if err != nil {
		return err
	}
	taskInfo := aria.WaitForTaskResult(ctx, vc, resp.Returnval)
	if taskInfo.Error != nil {
		return fmt.Errorf(taskInfo.Error.LocalizedMessage)
	}
	return nil
}

// ReserveCompute reserves given cpu and memory fully
// Shares of reservation is set to normal and currently changing is not
// supported. No NUMA node is configured either.
func ReserveCompute(ctx context.Context, vc *govmomi.Client, vm mo.VirtualMachine, memInMB int64, numCPUs int32, expandableMemory bool) error {
	var unlimited int64 = -1
	cpuReserve := int64(numCPUs * 1000)
	req := types.ReconfigVM_Task{
		This: vm.Self,
		Spec: types.VirtualMachineConfigSpec{
			CpuAllocation: &types.ResourceAllocationInfo{
				Reservation: types.NewInt64(cpuReserve), // 12000
				Shares: &types.SharesInfo{
					Level: types.SharesLevelNormal,
				},
				Limit: &unlimited,
			},
			MemoryAllocation: &types.ResourceAllocationInfo{
				Reservation:           &memInMB, // 49152
				ExpandableReservation: &expandableMemory,
				Limit:                 &unlimited,
				Shares: &types.SharesInfo{
					Level:  types.SharesLevelCustom,
					Shares: int32(memInMB),
				},
			},
		},
	}
	resp, err := methods.ReconfigVM_Task(ctx, vc, &req)
	if err != nil {
		return err
	}
	taskInfo := aria.WaitForTaskResult(ctx, vc, resp.Returnval)
	if taskInfo.Error != nil || taskInfo.Cancelled {
		return fmt.Errorf(taskInfo.Error.LocalizedMessage)
	}
	return nil
}

// ShutDown shutsdown a VM gracefully
func ShutDown(ctx context.Context, vc *govmomi.Client, computeData Compute, vmRef mo.VirtualMachine) error {
	req := &types.ShutdownGuest{
		This: vmRef.Self,
	}
	resp, err := methods.ShutdownGuest(ctx, vc, req)
	if err != nil {
		return err
	}
	fmt.Println(resp)
	taskInfo := aria.WaitForTaskResult(ctx, vc, types.ManagedObjectReference{})
	if taskInfo.Error != nil {
		return fmt.Errorf(taskInfo.Error.LocalizedMessage)
	}
	return nil
}
